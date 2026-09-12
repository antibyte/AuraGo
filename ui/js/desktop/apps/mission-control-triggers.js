// Mission Control – trigger catalog, human summaries, picker and config panel.
// Exposes window.MissionControlTriggers; DOM helpers receive deps from the shell.
// All DOM is built with createElement/textContent (no HTML string injection);
// trusted inline SVG icon strings from deps.svg are parsed as XML and imported.
(function () {
    'use strict';

    const TYPES = [
        { key: 'mission_completed', icon: 'check', labelKey: 'missions.trigger_mission_completed', hintKey: 'desktop.mc_trigger_mission_completed_hint' },
        { key: 'system_startup', icon: 'power', labelKey: 'missions.trigger_system_startup', hintKey: 'missions.trigger_system_startup_hint' },
        { key: 'budget_warning', icon: 'wallet', labelKey: 'missions.trigger_budget_warning', hintKey: 'missions.trigger_budget_warning_hint' },
        { key: 'budget_exceeded', icon: 'walletOff', labelKey: 'missions.trigger_budget_exceeded', hintKey: 'missions.trigger_budget_exceeded_hint' },
        { key: 'email_received', icon: 'mail', labelKey: 'missions.trigger_email_received', hintKey: 'desktop.mc_trigger_email_received_hint' },
        { key: 'webhook', icon: 'webhook', labelKey: 'missions.trigger_webhook', hintKey: 'missions.trigger_webhook_hint' },
        { key: 'fritzbox_call', icon: 'phone', labelKey: 'missions.trigger_fritzbox_call', hintKey: 'missions.trigger_fritzbox_call_hint' },
        { key: 'mqtt_message', icon: 'radio', labelKey: 'missions.trigger_mqtt_message', hintKey: 'missions.trigger_mqtt_hint' },
        { key: 'home_assistant_state', icon: 'home', labelKey: 'missions.trigger_home_assistant_state', hintKey: 'missions.trigger_home_assistant_state_hint' },
        { key: 'device_connected', icon: 'plug', labelKey: 'missions.trigger_device_connected', hintKey: 'missions.trigger_device_connected_hint' },
        { key: 'device_disconnected', icon: 'plugOff', labelKey: 'missions.trigger_device_disconnected', hintKey: 'missions.trigger_device_disconnected_hint' },
        { key: 'planner_appointment_due', icon: 'calendar', labelKey: 'missions.trigger_planner_appointment_due', hintKey: 'missions.trigger_planner_appointment_due_hint' },
        { key: 'planner_todo_overdue', icon: 'listCheck', labelKey: 'missions.trigger_planner_todo_overdue', hintKey: 'missions.trigger_planner_todo_overdue_hint' },
        { key: 'planner_operational_issue', icon: 'alert', labelKey: 'missions.trigger_planner_operational_issue', hintKey: 'missions.trigger_planner_operational_issue_hint' },
        { key: 'egg_hatched', icon: 'egg', labelKey: 'missions.trigger_egg_hatched', hintKey: 'missions.trigger_egg_hatched_hint' },
        { key: 'nest_cleared', icon: 'nest', labelKey: 'missions.trigger_nest_cleared', hintKey: 'missions.trigger_nest_cleared_hint' }
    ];

    const GROUPS = [
        { id: 'missions', labelKey: 'desktop.mc_trigger_group_missions', triggers: ['mission_completed', 'system_startup', 'budget_warning', 'budget_exceeded'] },
        { id: 'communication', labelKey: 'desktop.mc_trigger_group_communication', triggers: ['email_received', 'webhook', 'fritzbox_call'] },
        { id: 'devices', labelKey: 'desktop.mc_trigger_group_devices', triggers: ['mqtt_message', 'home_assistant_state', 'device_connected', 'device_disconnected'] },
        { id: 'planner', labelKey: 'desktop.mc_trigger_group_planner', triggers: ['planner_appointment_due', 'planner_todo_overdue', 'planner_operational_issue'] },
        { id: 'invasion', labelKey: 'desktop.mc_trigger_group_invasion', triggers: ['egg_hatched', 'nest_cleared'] }
    ];

    const REMOTE_ALLOWED = new Set(['system_startup', 'mqtt_message', 'home_assistant_state']);

    const text = (name, labelKey, extra) => Object.assign({ name, type: 'text', labelKey }, extra || {});
    const FIELDS = {
        mission_completed: [
            { name: 'source_mission_id', type: 'mission', labelKey: 'missions.trigger_source_mission_label', nameField: 'source_mission_name' },
            { name: 'require_success', type: 'toggle', labelKey: 'missions.trigger_require_success' }
        ],
        email_received: [
            { name: 'email_folder', type: 'select', labelKey: 'missions.trigger_email_folder_label', options: [{ value: 'INBOX', labelKey: 'missions.trigger_email_folder_inbox' }, { value: 'Sent', labelKey: 'missions.trigger_email_folder_sent' }] },
            text('email_subject_contains', 'missions.trigger_email_subject_label', { placeholderKey: 'missions.trigger_email_subject_placeholder' }),
            text('email_from_contains', 'missions.trigger_email_from_label', { placeholderKey: 'missions.trigger_email_from_placeholder' })
        ],
        webhook: [{ name: 'webhook_id', type: 'remote', source: 'webhooks', labelKey: 'missions.trigger_webhook_label', hintKey: 'missions.trigger_webhook_hint', required: true, nameField: 'webhook_slug', emptyKey: 'missions.trigger_webhook_none' }],
        egg_hatched: [
            { name: 'egg_id', type: 'remote', source: 'eggs', labelKey: 'missions.trigger_egg_select_label', anyKey: 'missions.trigger_egg_any', nameField: 'egg_name' },
            { name: 'nest_id', type: 'remote', source: 'nests', labelKey: 'missions.trigger_nest_select_label', anyKey: 'missions.trigger_nest_any', nameField: 'nest_name' }
        ],
        nest_cleared: [{ name: 'nest_id', type: 'remote', source: 'nests', labelKey: 'missions.trigger_nest_select_label', anyKey: 'missions.trigger_nest_any', nameField: 'nest_name' }],
        mqtt_message: [
            text('mqtt_topic', 'missions.trigger_mqtt_topic_label', { placeholderKey: 'missions.trigger_mqtt_topic_placeholder', required: true }),
            text('mqtt_payload_contains', 'missions.trigger_mqtt_payload_label', { placeholderKey: 'missions.trigger_mqtt_payload_placeholder' }),
            { name: 'mqtt_min_interval_seconds', type: 'number', labelKey: 'missions.trigger_mqtt_min_interval_label' }
        ],
        home_assistant_state: [
            text('ha_entity_id', 'missions.trigger_ha_entity_id_label', { placeholderKey: 'missions.trigger_ha_entity_id_placeholder', required: true }),
            text('ha_state_equals', 'missions.trigger_ha_state_equals_label', { placeholderKey: 'missions.trigger_ha_state_equals_placeholder' })
        ],
        device_connected: [text('device_id', 'missions.trigger_device_id_label'), text('device_name', 'missions.trigger_device_name_label')],
        device_disconnected: [text('device_id', 'missions.trigger_device_id_label'), text('device_name', 'missions.trigger_device_name_label')],
        fritzbox_call: [{ name: 'call_type', type: 'select', labelKey: 'missions.trigger_fritzbox_call_type_label', options: [{ value: '', labelKey: 'missions.trigger_fritzbox_call_type_any' }, { value: 'call', labelKey: 'missions.trigger_fritzbox_call_type_call' }, { value: 'tam_message', labelKey: 'missions.trigger_fritzbox_call_type_tam' }] }],
        planner_appointment_due: [text('planner_appointment_id', 'missions.trigger_planner_appointment_id_label'), text('planner_title_contains', 'missions.trigger_planner_title_contains_label')],
        planner_todo_overdue: [text('planner_todo_id', 'missions.trigger_planner_todo_id_label'), text('planner_title_contains', 'missions.trigger_planner_title_contains_label')],
        planner_operational_issue: [
            text('planner_issue_source', 'missions.trigger_planner_issue_source_label', { placeholderKey: 'missions.trigger_planner_issue_source_placeholder' }),
            { name: 'planner_issue_severity', type: 'select', labelKey: 'missions.trigger_planner_issue_severity_label', options: [{ value: '', labelKey: 'missions.trigger_planner_issue_severity_any' }, { value: 'warning', labelKey: 'missions.trigger_planner_issue_severity_warning' }, { value: 'error', labelKey: 'missions.trigger_planner_issue_severity_error' }] },
            text('planner_title_contains', 'missions.trigger_planner_title_contains_label')
        ],
        system_startup: [],
        budget_warning: [],
        budget_exceeded: []
    };

    const SOURCES = {
        webhooks: { url: '/api/webhooks', items: (d) => (Array.isArray(d) ? d : []), label: (w) => `${w.name || w.id} (${w.slug || ''})`, name: (w) => w.slug || '' },
        eggs: { url: '/api/invasion/eggs', items: (d) => (d && (d.eggs || (Array.isArray(d) ? d : []))) || [], label: (e) => e.name || e.id, name: (e) => e.name || '' },
        nests: { url: '/api/invasion/nests', items: (d) => (d && (d.nests || (Array.isArray(d) ? d : []))) || [], label: (n) => n.name || n.id, name: (n) => n.name || '' }
    };

    function byKey(key) { return TYPES.find(type => type.key === key) || null; }

    function joinParts(parts) { return parts.filter(Boolean).join(' · '); }

    function detail(mission, t) {
        const cfg = (mission && mission.trigger_config) || {};
        const parts = [];
        switch (mission && mission.trigger_type) {
            case 'mission_completed': {
                const src = cfg.source_mission_name || cfg.source_mission_id || t('missions.trigger_info_unknown_mission');
                parts.push(t('missions.trigger_info_when_completed', { name: src }) + (cfg.require_success ? ' ' + t('missions.trigger_info_only_on_success') : ''));
                break;
            }
            case 'email_received':
                if (cfg.email_folder) parts.push(t('missions.trigger_info_folder_prefix') + ' ' + cfg.email_folder);
                if (cfg.email_subject_contains) parts.push(t('missions.trigger_info_subject_prefix') + ' "' + cfg.email_subject_contains + '"');
                if (cfg.email_from_contains) parts.push(t('missions.trigger_info_from_prefix') + ' "' + cfg.email_from_contains + '"');
                if (!parts.length) parts.push(t('missions.trigger_info_any_email'));
                break;
            case 'webhook': parts.push(t('missions.trigger_info_webhook_prefix') + ' ' + (cfg.webhook_slug || cfg.webhook_id || t('missions.trigger_info_webhook_unknown'))); break;
            case 'egg_hatched':
                parts.push(cfg.egg_name || cfg.egg_id ? t('missions.trigger_info_egg_prefix') + ' ' + (cfg.egg_name || cfg.egg_id) : t('missions.trigger_info_any_egg'));
                if (cfg.nest_name || cfg.nest_id) parts.push(t('missions.trigger_info_nest_prefix') + ' ' + (cfg.nest_name || cfg.nest_id));
                break;
            case 'nest_cleared': parts.push(cfg.nest_name || cfg.nest_id ? t('missions.trigger_info_nest_prefix') + ' ' + (cfg.nest_name || cfg.nest_id) : t('missions.trigger_info_any_nest')); break;
            case 'mqtt_message':
                parts.push(t('missions.trigger_info_mqtt_topic_prefix') + ' ' + (cfg.mqtt_topic || '#'));
                if (cfg.mqtt_payload_contains) parts.push(t('missions.trigger_info_mqtt_payload_prefix') + ' "' + cfg.mqtt_payload_contains + '"');
                break;
            case 'system_startup': parts.push(t('missions.trigger_system_startup_badge')); break;
            case 'home_assistant_state':
                parts.push(t('missions.trigger_info_ha_entity_prefix') + ' ' + (cfg.ha_entity_id || t('missions.trigger_info_ha_any_entity')));
                if (cfg.ha_state_equals) parts.push(t('missions.trigger_info_ha_state_prefix') + ' "' + cfg.ha_state_equals + '"');
                break;
            case 'device_connected': parts.push(t('missions.trigger_info_device_connected_prefix') + ' ' + (cfg.device_name || cfg.device_id || t('missions.trigger_info_any_device'))); break;
            case 'device_disconnected': parts.push(t('missions.trigger_info_device_disconnected_prefix') + ' ' + (cfg.device_name || cfg.device_id || t('missions.trigger_info_any_device'))); break;
            case 'fritzbox_call': parts.push(t('missions.trigger_info_fritzbox_prefix') + ' ' + (cfg.call_type || t('missions.trigger_info_fritzbox_any'))); break;
            case 'budget_warning': parts.push(t('missions.trigger_budget_warning_badge')); break;
            case 'budget_exceeded': parts.push(t('missions.trigger_budget_exceeded_badge')); break;
            case 'planner_appointment_due':
                if (cfg.planner_appointment_id) parts.push(t('missions.trigger_info_planner_appointment_id_prefix') + ' ' + cfg.planner_appointment_id);
                if (cfg.planner_title_contains) parts.push(t('missions.trigger_info_planner_title_prefix') + ' "' + cfg.planner_title_contains + '"');
                if (!parts.length) parts.push(t('missions.trigger_info_planner_any_appointment'));
                break;
            case 'planner_todo_overdue':
                if (cfg.planner_todo_id) parts.push(t('missions.trigger_info_planner_todo_id_prefix') + ' ' + cfg.planner_todo_id);
                if (cfg.planner_title_contains) parts.push(t('missions.trigger_info_planner_title_prefix') + ' "' + cfg.planner_title_contains + '"');
                if (!parts.length) parts.push(t('missions.trigger_info_planner_any_todo'));
                break;
            case 'planner_operational_issue':
                if (cfg.planner_issue_source) parts.push(t('missions.trigger_info_planner_issue_source_prefix') + ' ' + cfg.planner_issue_source);
                if (cfg.planner_issue_severity) parts.push(t('missions.trigger_info_planner_issue_severity_prefix') + ' ' + cfg.planner_issue_severity);
                if (!parts.length) parts.push(t('missions.trigger_info_planner_any_issue'));
                break;
            default: break;
        }
        if (cfg.min_interval_seconds > 0) {
            parts.push(t('missions.trigger_info_min_interval_prefix') + ' ' + t('desktop.rel_time_seconds', { count: cfg.min_interval_seconds }));
        }
        return joinParts(parts);
    }

    // Legacy missions.* trigger labels carry a leading emoji pictogram; the desktop
    // renders its own SVG icons, so only the words are used.
    function label(type, t) {
        return String(t(type.labelKey) || '').replace(/^[^\p{L}\p{N}]+/u, '').trim();
    }

    function summary(mission, t, ctx) {
        ctx = ctx || {};
        if (!mission) return '';
        if (mission.execution_type === 'scheduled') {
            if (!mission.schedule) return t('desktop.mc_overview_not_scheduled');
            return ctx.schedule ? ctx.schedule.describe(mission.schedule, t, ctx.lang) : mission.schedule;
        }
        if (mission.execution_type !== 'triggered') return t('desktop.mc_editor_mode_manual_desc');
        const type = byKey(mission.trigger_type);
        const name = type ? label(type, t) : (mission.trigger_type || '');
        const extra = detail(mission, t);
        if (!extra) return name;
        // Legacy detail strings may already start with the type name ("Webhook: digest"); avoid "Webhook · Webhook: digest".
        if (name && extra.toLowerCase().startsWith(name.toLowerCase())) return extra;
        return name + ' · ' + extra;
    }

    function emitter() {
        const handlers = new Map();
        return {
            on(name, cb) { if (!handlers.has(name)) handlers.set(name, new Set()); handlers.get(name).add(cb); return () => handlers.get(name).delete(cb); },
            emit(name, payload) { const set = handlers.get(name); if (set) set.forEach(cb => { try { cb(payload); } catch (err) { console.error('MissionControlTriggers handler failed', err); } }); },
            clear() { handlers.clear(); }
        };
    }

    // ── DOM helpers (createElement/textContent only) ──
    function make(tag, className, attrs) {
        const node = document.createElement(tag);
        if (className) node.className = className;
        if (attrs) Object.keys(attrs).forEach(name => { if (attrs[name] != null) node.setAttribute(name, String(attrs[name])); });
        return node;
    }

    function textEl(tag, className, value) {
        const node = make(tag, className);
        node.textContent = value == null ? '' : String(value);
        return node;
    }

    function option(opt, selected) {
        const node = make('option', '', { value: opt.value == null ? '' : opt.value, 'data-name': opt.name || '' });
        node.textContent = opt.label == null ? '' : String(opt.label);
        if (String(opt.value) === String(selected)) node.selected = true;
        return node;
    }

    // Trusted inline SVG strings from the shell icon map are parsed as XML (inert,
    // no script execution) and cached; anything that is not a well-formed <svg>
    // root falls back to a blank placeholder span so layout stays stable.
    const iconCache = new Map();
    function parseSvg(markup) {
        if (typeof markup !== 'string' || !markup.trim()) return null;
        if (iconCache.has(markup)) return iconCache.get(markup);
        let node = null;
        try {
            const doc = new DOMParser().parseFromString(markup.trim(), 'image/svg+xml');
            const root = doc.documentElement;
            if (root && root.localName === 'svg' && !doc.querySelector('parsererror')) node = root;
        } catch (_) { node = null; }
        iconCache.set(markup, node);
        return node;
    }

    function icon(svg, name) {
        const template = parseSvg(svg && svg[name]);
        if (template) {
            const node = document.importNode(template, true);
            node.setAttribute('aria-hidden', 'true');
            return node;
        }
        return make('span', 'vd-mc-icon-blank', { 'aria-hidden': 'true' });
    }

    // Picker: current selection card + expandable searchable grouped grid.
    function createPicker(deps) {
        const { t, svg } = deps;
        const events = emitter();
        let value = '';
        let remote = false;
        let open = false;
        let query = '';
        const element = make('div', 'vd-mc-trigger-picker');

        function optionButton(type) {
            const disabled = remote && !REMOTE_ALLOWED.has(type.key);
            const active = type.key === value;
            const button = make('button', 'vd-mc-trigger-option' + (active ? ' is-active' : ''), { type: 'button', role: 'option', 'aria-selected': active, 'data-mc-trigger': type.key });
            if (disabled) { button.disabled = true; button.title = t('desktop.mc_trigger_remote_unavailable'); }
            const iconWrap = make('span', 'vd-mc-trigger-option-icon');
            iconWrap.append(icon(svg, type.icon));
            const textWrap = make('span', 'vd-mc-trigger-option-text');
            textWrap.append(textEl('span', 'vd-mc-trigger-option-label', label(type, t)), textEl('span', 'vd-mc-trigger-option-hint', t(type.hintKey)));
            button.append(iconWrap, textWrap);
            return button;
        }

        function matches(type) {
            if (!query) return true;
            const hay = (label(type, t) + ' ' + t(type.hintKey) + ' ' + type.key).toLowerCase();
            return hay.includes(query);
        }

        function groupNodes() {
            const nodes = [];
            GROUPS.forEach(group => {
                const types = group.triggers.map(byKey).filter(type => type && matches(type));
                if (!types.length) return;
                const wrap = make('div', 'vd-mc-trigger-group', { role: 'group', 'aria-label': t(group.labelKey) });
                const grid = make('div', 'vd-mc-trigger-grid', { role: 'listbox' });
                types.forEach(type => grid.append(optionButton(type)));
                wrap.append(textEl('div', 'vd-mc-trigger-group-title', t(group.labelKey)), grid);
                nodes.push(wrap);
            });
            if (!nodes.length) nodes.push(textEl('div', 'vd-mc-trigger-empty', t('desktop.mc_trigger_no_match')));
            return nodes;
        }

        function currentButton() {
            const current = byKey(value);
            const button = make('button', 'vd-mc-trigger-current' + (current ? '' : ' is-empty'), { type: 'button', 'data-mc-trigger-toggle': '', 'aria-expanded': open });
            const iconWrap = make('span', 'vd-mc-trigger-current-icon');
            iconWrap.append(icon(svg, current ? current.icon : 'bolt'));
            const textWrap = make('span', 'vd-mc-trigger-current-text');
            textWrap.append(textEl('span', 'vd-mc-trigger-current-label', current ? label(current, t) : t('desktop.mc_trigger_none_selected')));
            if (current) textWrap.append(textEl('span', 'vd-mc-trigger-current-hint', t(current.hintKey)));
            const action = textEl('span', 'vd-mc-trigger-current-action', t('desktop.mc_trigger_change'));
            action.append(icon(svg, open ? 'chevronUp' : 'chevronDown'));
            button.append(iconWrap, textWrap, action);
            return button;
        }

        function searchLabel() {
            const label = make('label', 'vd-mc-trigger-search');
            const input = make('input', '', { type: 'search', 'data-mc-trigger-search': '', placeholder: t('desktop.mc_trigger_search'), 'aria-label': t('desktop.mc_trigger_search'), inputmode: 'search', enterkeyhint: 'search', autocomplete: 'off' });
            input.value = query;
            label.append(icon(svg, 'search'), input);
            return label;
        }

        function render() {
            const panel = make('div', 'vd-mc-trigger-panel');
            panel.hidden = !open;
            panel.append(searchLabel(), ...groupNodes());
            element.replaceChildren(currentButton(), panel);
        }

        // Re-render only the grouped results so the search input keeps focus and caret.
        function renderResults() {
            const panel = element.querySelector('.vd-mc-trigger-panel');
            if (!panel) return;
            const scroll = panel.scrollTop;
            Array.from(panel.children).forEach(child => { if (!child.classList.contains('vd-mc-trigger-search')) child.remove(); });
            panel.append(...groupNodes());
            panel.scrollTop = scroll;
        }

        element.addEventListener('click', (event) => {
            const toggle = event.target.closest('[data-mc-trigger-toggle]');
            if (toggle) { open = !open; render(); if (open) focusSearch(); return; }
            const optionEl = event.target.closest('[data-mc-trigger]');
            if (optionEl && !optionEl.disabled) {
                value = optionEl.dataset.mcTrigger;
                open = false;
                query = '';
                render();
                events.emit('change', value);
            }
        });
        element.addEventListener('input', (event) => {
            const input = event.target.closest('[data-mc-trigger-search]');
            if (!input) return;
            query = input.value.trim().toLowerCase();
            renderResults();
        });
        element.addEventListener('keydown', (event) => {
            if (event.key === 'Escape' && open) { event.stopPropagation(); open = false; render(); const toggle = element.querySelector('[data-mc-trigger-toggle]'); if (toggle) toggle.focus(); }
        });

        function focusSearch() { const input = element.querySelector('[data-mc-trigger-search]'); if (input) input.focus(); }

        render();
        return {
            element,
            value() { return value; },
            setValue(key) { value = byKey(key) ? key : ''; open = !value; render(); },
            setRemote(flag) { remote = !!flag; if (remote && value && !REMOTE_ALLOWED.has(value)) { value = ''; events.emit('change', ''); } render(); },
            focusSearch,
            on: events.on,
            dispose() { events.clear(); element.replaceChildren(); }
        };
    }

    // Config panel: renders FIELDS for the active trigger plus the common minimum interval.
    function createConfigPanel(deps) {
        const { t, request, svg, missions } = deps;
        const events = emitter();
        const cache = {};
        let trigger = '';
        let config = {};
        let missionId = '';
        let renderToken = 0;
        const element = make('div', 'vd-mc-trigger-config');

        function fieldLabel(field, id) {
            const label = textEl('label', 'vd-mc-field-label', t(field.labelKey));
            label.setAttribute('for', id);
            if (field.required) {
                const mark = textEl('span', 'vd-mc-required', '*');
                mark.setAttribute('aria-hidden', 'true');
                label.append(' ', mark);
            }
            return label;
        }

        function fieldWrap(field, extraClass) {
            return make('div', 'vd-mc-field' + (extraClass ? ' ' + extraClass : ''), { 'data-mc-field-wrap': field.name });
        }

        function fieldHint(field) { return field.hintKey ? textEl('div', 'vd-mc-field-hint', t(field.hintKey)) : null; }

        function fieldError(field) {
            const node = make('div', 'vd-mc-field-error', { 'data-mc-error': field.name });
            node.hidden = true;
            return node;
        }

        function appendAll(parent, nodes) { nodes.forEach(node => { if (node) parent.append(node); }); return parent; }

        function fieldNode(field, value) {
            const id = `mc-tf-${field.name}`;
            switch (field.type) {
                case 'text': {
                    const input = make('input', 'vd-mc-input', { id, type: 'text', 'data-mc-field': field.name, placeholder: field.placeholderKey ? t(field.placeholderKey) : '', inputmode: 'text', enterkeyhint: 'done', autocomplete: 'off' });
                    input.value = value == null ? '' : String(value);
                    return appendAll(fieldWrap(field), [fieldLabel(field, id), input, fieldHint(field), fieldError(field)]);
                }
                case 'number': {
                    const input = make('input', 'vd-mc-input', { id, type: 'number', 'data-mc-field': field.name, min: '0', step: '1', inputmode: 'numeric', enterkeyhint: 'done' });
                    input.value = value == null ? '0' : String(value);
                    return appendAll(fieldWrap(field), [fieldLabel(field, id), input, fieldHint(field), fieldError(field)]);
                }
                case 'toggle': {
                    const label = make('label', 'vd-mc-toggle');
                    const input = make('input', '', { id, type: 'checkbox', 'data-mc-field': field.name });
                    input.checked = !!value;
                    label.append(input, make('span', 'vd-mc-toggle-track', { 'aria-hidden': 'true' }), textEl('span', 'vd-mc-toggle-text', t(field.labelKey)));
                    return appendAll(fieldWrap(field, 'vd-mc-field--toggle'), [label, fieldHint(field)]);
                }
                case 'select': {
                    const select = make('select', 'vd-mc-select', { id, 'data-mc-field': field.name });
                    const selected = value == null ? field.options[0].value : value;
                    field.options.forEach(opt => select.append(option({ value: opt.value, label: t(opt.labelKey) }, selected)));
                    return appendAll(fieldWrap(field), [fieldLabel(field, id), select, fieldHint(field), fieldError(field)]);
                }
                case 'remote': {
                    const select = make('select', 'vd-mc-select', { id, 'data-mc-field': field.name, 'data-mc-source': field.source });
                    select.disabled = true;
                    select.append(option({ value: '', label: t('desktop.mc_trigger_loading') }, ''));
                    return appendAll(fieldWrap(field), [fieldLabel(field, id), select, fieldHint(field), fieldError(field)]);
                }
                case 'mission': {
                    const list = (typeof missions === 'function' ? missions() : []).filter(m => m && m.id !== missionId && (m.execution_type === 'manual' || m.execution_type === 'scheduled'));
                    const group = make('div', 'vd-mc-mission-options', { role: 'radiogroup' });
                    if (list.length) {
                        list.forEach(m => {
                            const label = make('label', 'vd-mc-mission-option');
                            const radio = make('input', '', { type: 'radio', name: 'mc-source-mission', value: m.id, 'data-name': m.name, 'data-mc-field': field.name });
                            radio.checked = m.id === value;
                            label.append(radio, textEl('span', 'vd-mc-mission-option-name', m.name), textEl('span', 'vd-mc-mission-option-meta', t(m.execution_type === 'scheduled' ? 'missions.filter_scheduled' : 'missions.filter_manual')));
                            group.append(label);
                        });
                    } else {
                        group.append(textEl('div', 'vd-mc-field-hint', t('missions.trigger_no_suitable_missions')));
                    }
                    return appendAll(fieldWrap(field), [fieldLabel(field, id), group, fieldError(field)]);
                }
                default: return null;
            }
        }

        async function loadSource(name) {
            if (cache[name]) return cache[name];
            const source = SOURCES[name];
            const data = await request(source.url);
            cache[name] = source.items(data).map(item => ({ value: item.id, label: source.label(item), name: source.name(item) }));
            return cache[name];
        }

        async function fillRemote(token) {
            const selects = Array.from(element.querySelectorAll('select[data-mc-source]'));
            await Promise.all(selects.map(async (select) => {
                const field = (FIELDS[trigger] || []).find(f => f.name === select.dataset.mcField);
                if (!field) return;
                const selected = config[field.name] || '';
                try {
                    const options = await loadSource(field.source);
                    if (token !== renderToken) return;
                    const first = field.anyKey ? [{ value: '', label: t(field.anyKey), name: '' }] : (options.length ? [] : [{ value: '', label: t(field.emptyKey || 'desktop.mc_trigger_no_options'), name: '' }]);
                    select.replaceChildren(...first.concat(options).map(opt => option(opt, selected)));
                    if (selected && !options.some(opt => String(opt.value) === String(selected))) {
                        select.append(option({ value: selected, label: config[field.nameField] || selected, name: config[field.nameField] || '' }, selected));
                    }
                    select.disabled = false;
                } catch (_) {
                    if (token !== renderToken) return;
                    select.replaceChildren(option({ value: selected, label: t('desktop.mc_trigger_load_failed') }, selected));
                    select.disabled = false;
                }
            }));
        }

        function commonFields() {
            const wrap = make('div', 'vd-mc-field', { 'data-mc-field-wrap': 'min_interval_seconds' });
            const label = textEl('label', 'vd-mc-field-label', t('desktop.mc_editor_min_interval'));
            label.setAttribute('for', 'mc-tf-min-interval');
            const input = make('input', 'vd-mc-input', { id: 'mc-tf-min-interval', type: 'number', 'data-mc-field': 'min_interval_seconds', min: '0', max: '86400', step: '1', inputmode: 'numeric', enterkeyhint: 'done' });
            input.value = String(config.min_interval_seconds || 0);
            wrap.append(label, input, textEl('div', 'vd-mc-field-hint', t('missions.trigger_min_interval_hint')));
            return wrap;
        }

        function render() {
            const token = ++renderToken;
            if (!trigger) { element.replaceChildren(); return; }
            const fields = FIELDS[trigger] || [];
            const title = make('div', 'vd-mc-trigger-config-title');
            title.append(icon(svg, 'sliders'), textEl('span', '', t('desktop.mc_trigger_settings')));
            const nodes = [title];
            if (fields.length) {
                fields.forEach(field => { const node = fieldNode(field, config[field.name]); if (node) nodes.push(node); });
            } else {
                nodes.push(textEl('div', 'vd-mc-field-hint', t('desktop.mc_trigger_no_options')));
            }
            nodes.push(commonFields());
            element.replaceChildren(...nodes);
            fillRemote(token);
        }

        element.addEventListener('input', () => events.emit('change'));
        element.addEventListener('change', () => events.emit('change'));

        function fieldElement(field) {
            return element.querySelector(field.type === 'mission' ? `input[data-mc-field="${field.name}"]:checked` : `[data-mc-field="${field.name}"]`);
        }

        function readField(field) {
            const el = fieldElement(field);
            if (!el) return field.type === 'toggle' ? false : '';
            if (field.type === 'toggle') return !!el.checked;
            if (field.type === 'number') return parseInt(el.value || '0', 10) || 0;
            return String(el.value || '').trim();
        }

        function readName(field) {
            const el = fieldElement(field);
            if (!el) return '';
            if (el.tagName === 'SELECT') { const opt = el.options[el.selectedIndex]; return (opt && opt.dataset.name) || ''; }
            return el.dataset.name || '';
        }

        return {
            element,
            setTrigger(key, cfg, currentMissionId) {
                trigger = byKey(key) ? key : '';
                config = Object.assign({}, cfg || {});
                missionId = currentMissionId || '';
                render();
            },
            getConfig() {
                const minInput = element.querySelector('[data-mc-field="min_interval_seconds"]');
                const out = { min_interval_seconds: minInput ? (parseInt(minInput.value || '0', 10) || 0) : 0 };
                (FIELDS[trigger] || []).forEach(field => {
                    out[field.name] = readField(field);
                    if (field.nameField) out[field.nameField] = readName(field);
                });
                return out;
            },
            validate() {
                const errors = [];
                (FIELDS[trigger] || []).forEach(field => {
                    const wrap = element.querySelector(`[data-mc-field-wrap="${field.name}"]`);
                    const errorEl = element.querySelector(`[data-mc-error="${field.name}"]`);
                    const missing = !!field.required && !readField(field);
                    if (wrap) wrap.classList.toggle('has-error', missing);
                    if (errorEl) { errorEl.hidden = !missing; errorEl.textContent = missing ? t('desktop.mc_field_required') : ''; }
                    if (missing) errors.push({ field: field.name, messageKey: 'desktop.mc_field_required' });
                });
                return errors;
            },
            on: events.on,
            dispose() { events.clear(); element.replaceChildren(); }
        };
    }

    window.MissionControlTriggers = { GROUPS, TYPES, REMOTE_ALLOWED, FIELDS, byKey, label, summary, detail, createPicker, createConfigPanel };
})();
