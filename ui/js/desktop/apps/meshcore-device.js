(function () {
    'use strict';
    function create(s, h) {
        const { node, tr, iconEl, request, errorText, dialog, dialogButton } = h;
        const t = key => tr(s, key);
        const unknown = () => t('value_unknown');
        const date = n => n ? new Date(n * 1000).toLocaleString() : unknown();
        const yes = v => v == null ? unknown() : t(v ? 'enabled' : 'disabled');
        const clone = value => JSON.parse(JSON.stringify(value));
        const page = node('section', undefined, 'mc-device-page'); page.hidden = true;
        const nav = node('nav', undefined, 'mc-page-nav'), heading = node('h2'); heading.tabIndex = -1;
        const feedback = node('p', '', 'mc-feedback'); feedback.hidden = true; feedback.setAttribute('role', 'status');
        const devicePane = node('div', undefined, 'mc-device-grid'), settingsPane = node('div', undefined, 'mc-settings-layout');
        const settingsNav = node('nav', undefined, 'mc-settings-nav'), settingsSelect = node('select', undefined, 'mc-settings-select'), settingsContent = node('div', undefined, 'mc-settings-content'), recovery = node('div', undefined, 'mc-settings-recovery');
        settingsNav.setAttribute('aria-label', t('settings')); settingsSelect.setAttribute('aria-label', t('settings')); recovery.hidden = true;
        settingsSelect.addEventListener('change', () => selectSection(settingsSelect.value, true));
        settingsPane.append(recovery, settingsNav, settingsSelect, settingsContent);
        page.append(nav, heading, feedback, devicePane, settingsPane); s.root.append(page);
        let view = '', device, loading = false, lastRead = 0, jobTimer, activeSection = 'app';
        const forms = new Map();
        function button(parent, key, fn, disabled = false) {
            const b = node('button', t(key)); b.type = 'button'; b.disabled = disabled;
            b.addEventListener('click', async () => { try { await fn(); } catch (e) { show(feedback, errorText(s, e)); } }); parent.append(b); return b;
        }
        button(nav, 'back', close).prepend(iconEl('back'));
        function selectSection(section, focus = false) {
            activeSection = section; settingsSelect.value = section;
            settingsContent.querySelectorAll('[data-mc-section]').forEach(el => { el.hidden = el.dataset.mcSection !== section; });
            settingsNav.querySelectorAll('button').forEach(el => el.setAttribute('aria-pressed', String(el.dataset.mcSectionLink === section)));
            if (focus) { if (feedback.dataset.feedback === 'success') feedback.hidden = true; const title = settingsContent.querySelector(`[data-mc-section="${section}"] h3`); title?.focus({ preventScroll: true }); page.scrollTop = 0; }
        }
        const sectionIcons = { app: 'settings', identity: 'self', contacts: 'shield', radio: 'mesh', clock: 'clock' };
        function sectionLink(section, title) {
            const b = button(settingsNav, title, () => selectSection(section, true)); b.dataset.mcSectionLink = section;
            b.prepend(iconEl(sectionIcons[section]));
            settingsSelect.append(new Option(t(title), section));
            return b;
        }
        function syncNavigation(next) { s.root.querySelectorAll('[data-mc-view]').forEach(el => el.setAttribute('aria-pressed', String(el.dataset.mcView === next))); }
        function show(el, text, state = 'error') { if (!s.disposed) { el.hidden = false; el.textContent = text; el.dataset.feedback = state; } }
        function rows(parent, pairs) {
            const dl = node('dl', undefined, 'mc-data-grid');
            for (const [key, value] of pairs) dl.append(node('dt', t(key)), node('dd', value == null || value === '' ? unknown() : String(value)));
            parent.append(dl); return dl;
        }
        function group(parent, key) {
            const el = node('section', undefined, 'mc-device-card'), title = node('h3', t(key)); title.tabIndex = -1;
            title.prepend(iconEl({ self: 'self', radio: 'mesh', diagnostics: 'info', supported_features: 'check', storage: 'info', packets: 'send', device_clock: 'clock' }[key] || 'info'));
            el.append(title); parent.append(el); return el;
        }
        function position(p) { return p ? `${p.latitude}°, ${p.longitude}°` : unknown(); }
        function path(p) {
            if (!p) return unknown();
            return `${t(p.route === 'flood' ? 'route_flood' : p.route === 'direct' ? 'route_direct' : 'value_unknown')} · ${p.hops == null ? unknown() : p.hops + ' ' + t('hops')}${p.hash_bytes == null ? '' : ' · ' + p.hash_bytes + ' B'}${p.hashes ? ' · ' + p.hashes : ''}`;
        }
        function identity(parent, st) {
            rows(parent, [['name', st.name], ['identity', st.identity_key], ['firmware', st.firmware], ['manufacturer', st.device?.manufacturer], ['build', st.device?.build_date], ['protocol', st.device?.protocol_version], ['sampled_at', date(st.snapshot_at)]]);
        }
        function nodeHero(parent, st) {
            const hero = node('div', undefined, 'mc-node-hero'), mark = node('span', undefined, 'mc-node-mark'), copy = node('div', undefined, 'mc-node-copy');
            const known = ['connected', 'connecting', 'disconnected', 'disabled', 'binding_required', 'binding_changed', 'updating', 'suspended', 'settings_uncertain'].includes(st.state) ? st.state : 'disconnected';
            mark.dataset.state = known; mark.setAttribute('aria-hidden', 'true'); mark.append(iconEl('mesh'));
            const state = node('span', t('state_' + known), 'mc-status'); state.dataset.state = known;
            const key = node('code', st.identity_key || unknown(), 'mc-node-key');
            const chips = node('div', undefined, 'mc-chips');
            for (const value of [st.firmware, st.device?.manufacturer]) if (value) chips.append(node('span', String(value), 'mc-chip'));
            copy.append(node('strong', st.name || unknown(), 'mc-node-title'), state, key, chips);
            hero.append(mark, copy); parent.append(hero);
        }
        // One-line radio signature in the notation operators use on air: frequency, SF, bandwidth, CR, power.
        function radioSignature(parent, r) {
            if (!r) return;
            const sig = node('div', undefined, 'mc-radio-sig');
            const mhz = Number.isFinite(r.frequency_khz) ? (r.frequency_khz / 1000).toFixed(3) : null;
            const freq = node('strong', mhz ?? unknown(), 'mc-radio-freq'); if (mhz) freq.append(node('small', 'MHz'));
            const chips = node('div', undefined, 'mc-chips');
            for (const value of [r.spreading_factor != null && 'SF' + r.spreading_factor, r.bandwidth_hz != null && (r.bandwidth_hz >= 1000 ? r.bandwidth_hz / 1000 + ' kHz' : r.bandwidth_hz + ' Hz'), r.coding_rate_denominator != null && 'CR 4/' + r.coding_rate_denominator, r.tx_power_dbm != null && r.tx_power_dbm + ' dBm']) if (value) chips.append(node('span', value, 'mc-chip'));
            sig.append(freq, chips); parent.append(sig);
        }
        function radioRows(parent, r) {
            if (!r) return;
            rows(parent, [['position', position(r.configured_position)], ['frequency', r.frequency_khz + ' kHz'], ['bandwidth', r.bandwidth_hz + ' Hz'], ['spreading_factor', r.spreading_factor], ['coding_rate', '4/' + r.coding_rate_denominator], ['tx_power', r.tx_power_dbm + ' dBm'], ['multi_acks', r.multi_acks]]);
        }
        function renderDevice() {
            devicePane.replaceChildren();
            if (!device) { devicePane.append(node('p', t('offline_hint'))); return; }
            const st = device.status;
            const profile = group(devicePane, 'self'); profile.classList.add('mc-device-profile'); nodeHero(profile, st);
            rows(profile, [['firmware', st.firmware], ['manufacturer', st.device?.manufacturer], ['build', st.device?.build_date], ['protocol', st.device?.protocol_version], ['sampled_at', date(st.snapshot_at)]]);
            const actions = node('div', undefined, 'mc-actions'); profile.append(actions);
            button(actions, 'share', () => h.selfDialog(s), st.state !== 'connected' || !!s.context.readonly).prepend(iconEl('share'));
            const stats = node('div', undefined, 'mc-device-stats'); devicePane.append(stats);
            for (const [key, value, used, total] of [['contacts_capacity', `${st.contacts?.length ?? unknown()} / ${st.device?.contact_capacity ?? unknown()}`, st.contacts?.length, st.device?.contact_capacity], ['channels_capacity', `${st.channels?.length ?? unknown()} / ${st.channel_capacity ?? unknown()}`, st.channels?.length, st.channel_capacity], ['device_clock', date(device.clock)]]) {
                const stat = node('div', undefined, 'mc-stat'); stat.append(node('span', t(key), 'mc-stat-label'), node('strong', value, 'mc-stat-value'));
                if (Number.isFinite(used) && total > 0) { const meter = node('span', undefined, 'mc-meter'); meter.setAttribute('aria-hidden', 'true'); meter.style.setProperty('--mc-fill', Math.min(100, used / total * 100).toFixed(1) + '%'); meter.classList.toggle('mc-meter-high', used / total >= 0.85); stat.append(meter); }
                stats.append(stat);
            }
            const radio = group(devicePane, 'radio'); radioSignature(radio, st.radio); radioRows(radio, st.radio);
            const features = group(devicePane, 'supported_features');
            const labels = { identity: 'name', radio: 'radio', clock: 'device_clock', other: 'telemetry', auto_add: 'auto_add', auto_add_max_hops: 'max_hops', repeat: 'repeat', path_hash: 'path_hash', multi_ack: 'multi_acks' };
            rows(features, Object.entries(device.features || {}).map(([k, v]) => [labels[k] || 'type', t(v === 'available' ? 'enabled' : v === 'unsupported' ? 'unsupported' : 'value_unknown')]));
            const diagnostics = group(devicePane, 'diagnostics'); diagnostics.classList.add('mc-device-diagnostics'); diagnostics.append(node('p', t('local_hint'), 'mc-hint'));
            const readings = node('div', undefined, 'mc-diagnostic-grid'); diagnostics.append(readings);
            if (!device.local) diagnostics.append(node('p', unknown()));
            for (const [name, g] of Object.entries(device.local?.groups || {})) {
                const card = group(readings, name === 'radio' ? 'radio' : name === 'storage' ? 'storage' : name === 'core' ? 'self' : 'packets');
                rows(card, [['sampled_at', date(g.at)]]);
                if (g.state !== 'available') { card.append(node('p', t(g.state === 'unsupported' ? 'unsupported' : 'value_unknown'))); continue; }
                const units = { battery_mv: 'mV', storage_used_kb: 'KiB', storage_total_kb: 'KiB', uptime_seconds: 's', noise_floor_dbm: 'dBm', last_rssi_dbm: 'dBm', last_snr_db: 'dB', tx_airtime_seconds: 's', rx_airtime_seconds: 's' };
                rows(card, Object.entries(g.values || {}).map(([key, value]) => [key, `${value}${units[key] ? ' ' + units[key] : ''}`]));
            }
        }
        function messageDetails(msg) {
            const d = dialog(s, 'details', 'info'), m = msg.details, rx = m?.reception;
            rows(d.body, [[msg.direction === 'incoming' ? 'received_at' : 'created_at', date(m?.received_at || msg.at)]]);
            if (msg.direction === 'incoming') rows(d.body, [['sender_time', date(m?.sender_timestamp)], ['text_type', m?.text_type], ['route', rx ? path(rx.path) : null], ['snr', rx?.snr_db == null ? null : rx.snr_db + ' dB'], ['frame', rx ? `${rx.frame_type} · ${rx.companion_frame_bytes} B` : null], ['forwarded_prefix', rx?.forwarded_sender_prefix], ['reserved', rx?.reserved_hex]]);
            if (m?.sender_label && !msg.protected) rows(d.body, [['sender_label', m.sender_label]]);
            if (m?.sender_contact) contactRows(group(d.body, 'contact_snapshot'), m.sender_contact);
            if (m?.receiver) { const g = group(d.body, 'receiver_snapshot'); identity(g, m.receiver); radioRows(g, m.receiver.radio); }
            for (const p of msg.parts || []) rows(group(d.body, 'send'), [['packet', p.number], ['route', p.route == null ? null : t(p.route === 1 ? 'route_flood' : p.route === 0 ? 'route_direct' : 'value_unknown')], ['ack_duration', p.ack_millis == null ? null : p.ack_millis + ' ms']]);
            d.body.append(node('p', t('metrics_hint'), 'mc-hint'));
        }
        function contactRows(parent, c) {
            const types = { 1: 'companion', 2: 'repeater', 3: 'room', 4: 'sensor' };
            rows(parent, [['type', t(types[c.type] || 'value_unknown')], ['device_flags', c.flags], ['device_favorite', yes(c.flags == null ? null : (c.flags & 1) !== 0)], ['position', position(c.advertised_position)], ['last_advert', date(c.last_advert_timestamp)], ['last_modified', date(c.last_modified_timestamp)], ['outgoing_path', path(c.out_path)]]);
        }
        function contactDetails(parent, c) {
            rows(parent, [['messenger_favorite', yes(c.favorite)]]);
            if (c.kind === 'channel') { rows(parent, [['channel_slot', c.channel], ['binding', t(c.active ? 'state_connected' : 'archived')]]); return; }
            const sameDevice = c.identity_key === s.status.identity_key;
            const contact = sameDevice && s.status.contacts?.find(item => item.key === c.target);
            rows(parent, [['agent_trusted', yes(sameDevice && s.status.state === 'connected' && Array.isArray(s.settings?.trusted_nodes) ? s.settings.trusted_nodes.includes(c.target) : null)]]);
            if (contact) contactRows(parent, contact);
            if (c.active && contact) {
                const actions = node('div', undefined, 'mc-detail-actions'); parent.append(actions);
                for (const kind of ['telemetry', 'path']) button(actions, kind === 'path' ? 'discover_path' : 'get_telemetry', () => diagnose(c.target, kind), !s.settings?.allow_remote_diagnostics || s.status.state !== 'connected' || !!s.context.readonly);
                if (!s.settings?.allow_remote_diagnostics) parent.append(node('p', t('remote_gate_hint'), 'mc-hint'));
            }
        }
        async function diagnose(target, kind) {
            const d = dialog(s, kind === 'path' ? 'discover_path' : 'get_telemetry', 'info');
            const output = node('div'); d.body.append(node('p', target, 'mc-key'), output);
            show(output, t('diagnostic_wait'), 'pending');
            let job;
            try { job = await request(s, 'diagnostics', { identity: s.status.identity_key, target, kind }); }
            catch (e) { show(output, errorText(s, e)); return; }
            async function poll() {
                if (s.disposed || !d.el.isConnected) return;
                if (document.hidden || !s.root.getClientRects().length) { jobTimer = setTimeout(poll, 1500); return; }
                try {
                    const result = await request(s, 'diagnostics?id=' + encodeURIComponent(job.id));
                    if (s.disposed || !d.el.isConnected) return;
                    if (result.state === 'pending') { jobTimer = setTimeout(poll, 1000); return; }
                    output.replaceChildren();
                    rows(output, [['identity', result.target], ['sampled_at', date(result.completed_at)]]);
                    if (result.state === 'timeout') { output.append(node('p', t('no_response'))); return; }
                    if (result.state !== 'completed') { output.append(node('p', t('diagnostic_failed'))); return; }
                    if (kind === 'path') rows(output, [['outgoing_path', path(result.outgoing)], ['incoming_path', path(result.incoming)]]);
                    for (const v of result.telemetry || []) rows(output, [[v.type === 136 ? 'position' : 'sensor', `${v.channel} · ${v.values.join(', ')} ${v.unit}`]]);
                    if (kind === 'telemetry' && !result.telemetry?.length) output.append(node('p', unknown()));
                } catch (e) { show(output, errorText(s, e)); }
            }
            d.el.addEventListener('close', () => clearTimeout(jobTimer), { once: true }); await poll();
        }
        const sectionFields = {
            app: ['history_days', 'history_messages', 'allow_device_settings', 'allow_remote_diagnostics'],
            identity: ['name', 'latitude', 'longitude', 'advert_location_policy'],
            contacts: ['manual_add_contacts', 'auto_add_mask', 'auto_add_max_hops', 'telemetry_base', 'telemetry_location', 'telemetry_environment'],
            radio: ['frequency_khz', 'bandwidth_hz', 'spreading_factor', 'coding_rate', 'tx_power_dbm', 'multi_acks', 'repeat', 'path_hash_mode']
        };
        const fieldLabels = { frequency_khz: 'frequency', bandwidth_hz: 'bandwidth', tx_power_dbm: 'tx_power', auto_add_mask: 'auto_add', auto_add_max_hops: 'max_hops', path_hash_mode: 'path_hash' };
        function fieldLabel(key) { return fieldLabels[key] || key; }
        function source(section) { return section === 'app' ? s.settings : device?.values; }
        function controls(form, key, value) {
            const label = node('label', t(fieldLabel(key))), input = node('input'); input.autocomplete = 'off';
            let el = input;
            const choices = {
                advert_location_policy: [[0, t('location_none')], [1, t('location_current')]],
                manual_add_contacts: [[0, t('auto_all')], [1, t('auto_filtered')]],
                telemetry_base: [[0, t('disabled')], [1, t('permission_flags')], [2, t('all')]],
                spreading_factor: Array.from({length: 8}, (_, i) => [i + 5, 'SF ' + (i + 5)]),
                coding_rate: [5, 6, 7, 8].map(n => [n, '4/' + n]),
                bandwidth_hz: [7800, 10400, 15600, 20800, 31250, 41700, 62500, 125000, 250000, 500000].map(n => [n, n + ' Hz']),
                multi_acks: [0, 1, 2, 3].map(n => [n, String(n)]),
                path_hash_mode: [0, 1, 2].map(n => [n, (n + 1) + ' B'])
            };
            choices.auto_add_max_hops = Array.from({length: 65}, (_, n) => [n, n ? `${n - 1} ${t('hops')}` : t('unlimited')]);
            choices.telemetry_location = choices.telemetry_environment = choices.telemetry_base;
            if (key === 'auto_add_mask') {
                el = node('fieldset'); const legend = node('legend', t('auto_add')); el.append(legend); label.textContent = '';
                for (const [bit, title] of [[1, 'overwrite_contacts'], [2, 'companion'], [4, 'repeater'], [8, 'room'], [16, 'sensor']]) { const l = node('label', t(title)), c = node('input'); c.type = 'checkbox'; c.value = bit; l.prepend(c); el.append(l); }
            } else if (choices[key]) {
                el = node('select'); for (const [v, text] of choices[key]) el.append(new Option(text, v));
                if (value != null && !choices[key].some(([v]) => v === value)) el.append(new Option(String(value), value));
            } else if (typeof value === 'boolean' || key.startsWith('allow_') || key === 'repeat') input.type = 'checkbox';
            else if (key === 'name') { input.type = 'text'; input.maxLength = 31; input.required = true; }
            else {
                input.type = 'number'; input.required = true; input.step = '1';
                const limits = { history_days: [1, 3650], history_messages: [1, 100000], latitude: [-90, 90], longitude: [-180, 180], frequency_khz: [150000, 2500000], tx_power_dbm: [-9, device?.status.radio?.max_tx_power_dbm ?? 0], auto_add_max_hops: [0, 64] };
                if (limits[key]) { input.min = limits[key][0]; input.max = limits[key][1]; }
                if (key === 'latitude' || key === 'longitude') input.step = '0.000001';
            }
            if (el.type === 'checkbox') label.classList.add('mc-setting-toggle');
            if (key === 'auto_add_mask') label.classList.add('mc-setting-flags');
            el.dataset.mcField = key; label.append(el); form.fields.append(label); form.inputs[key] = el;
            return el;
        }
        function readInput(el, key) {
            if (key === 'auto_add_mask') return [...el.querySelectorAll('input:checked')].reduce((n, c) => n | Number(c.value), 0);
            return el.type === 'checkbox' ? el.checked : key === 'name' ? el.value : Number(el.value);
        }
        function fill(form) {
            const values = source(form.section); if (!values) return;
            form.base = clone(values); form.revision = form.section === 'app' ? s.settings.settings_revision : device.revision; form.identity = device?.status.identity_key;
            for (const [key, el] of Object.entries(form.inputs)) {
                const value = values[key];
                if (key === 'auto_add_mask') el.querySelectorAll('input').forEach(c => { c.checked = !!(value & Number(c.value)); });
                else if (el.type === 'checkbox') el.checked = !!value; else el.value = value ?? '';
            }
            form.dirty = false; form.marker.textContent = ''; updateLocks(form);
        }
        function updateLocks(form) {
            const app = form.section === 'app';
            form.fields.disabled = form.saving || !!s.context.readonly || (!app && (!device?.allow_device_settings || device.status.state !== 'connected'));
            const capability = { auto_add_mask: 'auto_add', auto_add_max_hops: 'auto_add_max_hops', repeat: 'repeat', path_hash_mode: 'path_hash', multi_acks: 'multi_ack', telemetry_base: 'other', telemetry_location: 'other', telemetry_environment: 'other', advert_location_policy: 'other' };
            for (const [key, el] of Object.entries(form.inputs)) if (capability[key]) {
                const state = device?.features[capability[key]];
                el.disabled = state !== 'available';
                let hint = el.parentElement.querySelector('[data-mc-capability]');
                if (!hint) { hint = node('small', '', 'mc-hint'); hint.dataset.mcCapability = key; el.parentElement.append(hint); }
                hint.hidden = !el.disabled; hint.textContent = t(state === 'unsupported' ? 'unsupported' : 'value_unknown');
            }
            form.save.disabled = form.fields.disabled || !form.dirty; form.discard.disabled = form.saving || !form.dirty;
            form.locked.hidden = !form.fields.disabled || form.saving;
            form.locked.textContent = t(s.context.readonly ? 'error_permission_denied' : !device?.allow_device_settings ? 'settings_locked' : device.status.state === 'settings_uncertain' ? 'state_settings_uncertain' : 'error_not_connected');
            const link = settingsNav.querySelector(`[data-mc-section-link="${form.section}"]`);
            link.classList.toggle('mc-section-dirty', form.dirty);
            settingsSelect.querySelector(`option[value="${form.section}"]`).textContent = link.textContent + (form.dirty ? ' · ' + t('unsaved') : '');
        }
        function makeForm(section, title) {
            const el = node('form', undefined, 'mc-device-card'), fields = node('fieldset', undefined, 'mc-settings-fields');
            const heading = node('h3', t(title)); heading.tabIndex = -1; heading.prepend(iconEl(sectionIcons[section]));
            el.dataset.mcSection = section; el.append(heading, fields);
            const form = { section, el, fields, inputs: {}, dirty: false, saving: false, marker: node('span', '', 'mc-unsaved'), locked: node('p', '', 'mc-settings-locked'), feedback: node('div', '', 'mc-feedback') };
            sectionLink(section, title);
            form.feedback.hidden = true; form.feedback.setAttribute('role', 'status');
            for (const key of sectionFields[section]) controls(form, key, source(section)?.[key]);
            if (section === 'identity') fields.append(node('p', t('location_hint'), 'mc-hint'));
            if (section === 'app') fields.append(node('p', t('gates_hint'), 'mc-hint'));
            const actions = node('div', undefined, 'mc-actions mc-settings-actions');
            form.save = button(actions, 'save', () => { if (el.reportValidity()) return submit(form); }); form.save.classList.add('mc-primary');
            form.discard = button(actions, 'discard', () => { fill(form); form.feedback.hidden = true; });
            actions.append(form.marker); el.append(form.locked, actions, form.feedback); settingsContent.append(el); forms.set(section, form);
            el.addEventListener('submit', e => { e.preventDefault(); if (!form.save.disabled && el.reportValidity()) submit(form); });
            fields.addEventListener('input', () => { form.dirty = sectionFields[section].some(k => !form.inputs[k].disabled && readInput(form.inputs[k], k) !== form.base?.[k]); form.marker.textContent = form.dirty ? t('unsaved') : ''; updateLocks(form); });
            fill(form); return form;
        }
        function renderSettings() {
            if (!forms.has('app')) makeForm('app', 'app_settings');
            if (s.status.state === 'binding_changed' && !settingsPane.querySelector('[data-mc-mapping]')) {
                const mapping = group(settingsPane, 'confirm_mapping'); mapping.dataset.mcMapping = 'true'; mapping.append(node('p', t('mapping_hint')));
                mapping.classList.add('mc-settings-recovery'); settingsPane.prepend(mapping);
                button(mapping, 'confirm_mapping', async () => { await h.manage(s, { action: 'confirm_mapping' }); mapping.remove(); await reload(); }, !!s.context.readonly);
            }
            if (device && !forms.has('identity')) {
                makeForm('identity', 'self'); makeForm('contacts', 'contacts_telemetry'); makeForm('radio', 'radio');
                const actions = group(settingsContent, 'device_clock'); actions.dataset.mcSection = 'clock'; actions.append(node('p', t('clock_hint'), 'mc-hint')); sectionLink('clock', 'device_clock');
                const sync = button(actions, 'sync_clock', () => singleAction('clock')); sync.dataset.mcDeviceAction = 'clock';
                recovery.append(node('p', t('reconcile_hint')));
                const reconcile = button(recovery, 'reconcile_settings', () => singleAction('reconcile')); reconcile.dataset.mcDeviceAction = 'reconcile';
            }
            for (const form of forms.values()) { if (!form.dirty && !form.saving) fill(form); else updateLocks(form); }
            settingsPane.querySelectorAll('[data-mc-device-action]').forEach(b => { const recovery = b.dataset.mcDeviceAction === 'reconcile'; b.hidden = recovery && device?.status.state !== 'settings_uncertain'; b.disabled = !!s.context.readonly || !device?.allow_device_settings || (recovery ? device.status.state !== 'settings_uncertain' : device.status.state !== 'connected' || device.features.clock !== 'available'); });
            recovery.hidden = device?.status.state !== 'settings_uncertain';
            selectSection(activeSection);
        }
        async function submit(form) {
            if (form.saving || form.save.disabled) return;
            const values = clone(form.base); for (const [key, el] of Object.entries(form.inputs)) if (!el.disabled) values[key] = readInput(el, key);
            const save = async () => {
                form.saving = true; updateLocks(form); show(form.feedback, t('state_updating'), 'pending');
                try {
                    if (form.section === 'app') { await request(s, 'settings', { ...Object.fromEntries(sectionFields.app.map(k => [k, values[k]])), revision: form.revision }); await h.refresh(s, false); if (device) { device.allow_device_settings = s.settings.allow_device_settings; device.allow_remote_diagnostics = s.settings.allow_remote_diagnostics; device.status = s.status; } }
                    else device = await request(s, 'device-settings', { identity: form.identity, revision: form.revision, section: form.section, values });
                    if (s.disposed) return;
                    fill(form); show(form.feedback, t('saved_verified'), 'success'); renderDevice(); renderSettings();
                } catch (e) {
                    if (e.device?.revision) { device = e.device; renderDevice(); }
                    show(form.feedback, errorText(s, e));
                    if (e.device?.revision && form.section !== 'app') rows(form.feedback, sectionFields[form.section].map(k => [fieldLabel(k), device.values[k]]));
                }
                finally { form.saving = false; if (!s.disposed) renderSettings(); }
            };
            if (form.section === 'radio') {
                const d = dialog(s, 'confirm', 'alert'); d.body.append(node('p', t('radio_warning')));
                rows(d.body, sectionFields.radio.filter(k => values[k] !== form.base[k]).map(k => [fieldLabel(k), `${form.base[k] ?? unknown()} → ${values[k] ?? unknown()}`]));
                dialogButton(s, d, 'confirm', async () => { d.el.close(); await save(); });
            } else await save();
        }
        async function singleAction(section) {
            const d = dialog(s, 'confirm', 'alert'); d.body.append(node('p', t(section === 'clock' ? 'clock_hint' : 'reconcile_hint')));
            const baseline = { identity: device.status.identity_key, revision: device.revision, section };
            dialogButton(s, d, 'confirm', async () => {
                try { device = await request(s, 'device-settings', baseline); }
                catch (e) { if (e.device?.revision) { device = e.device; renderDevice(); renderSettings(); } throw e; }
                d.el.close(); show(feedback, t('saved_verified'), 'success'); renderDevice(); renderSettings(); await h.refresh(s, false);
            });
        }
        async function reload() {
            if (loading || s.disposed) return; loading = true; lastRead = Date.now();
            try { device = await request(s, 'device'); if (s.disposed) return; renderDevice(); renderSettings(); }
            catch (e) { if (!s.disposed) { if (device) device.status.state = 'disconnected'; renderSettings(); show(feedback, errorText(s, e)); } }
            finally { loading = false; }
        }
        async function open(next) {
            view = next; page.hidden = false; s.root.classList.add('mc-subpage-open'); devicePane.hidden = next !== 'device'; settingsPane.hidden = next !== 'settings'; heading.textContent = t(next === 'device' ? 'device' : 'settings'); syncNavigation(next); page.scrollTop = 0; heading.focus({ preventScroll: true });
            renderSettings(); renderDevice();
            if (!device || Date.now() - lastRead >= 30000) await reload();
        }
        function close(focus = true) { view = ''; page.hidden = true; s.root.classList.remove('mc-subpage-open'); syncNavigation('messages'); h.updateComposer(s); if (focus) s.root.querySelector('[data-mc="messages"]').focus(); }
        const timer = setInterval(() => { if (view === 'device' && !document.hidden && page.getClientRects().length && Date.now() - lastRead >= 30000) reload(); }, 30000);
        return { open, close, reload, isOpen: () => !!view, messageDetails, contactDetails, dispose: () => { clearInterval(timer); clearTimeout(jobTimer); forms.clear(); device = null; page.remove(); } };
    }
    window.MeshCoreDevice = { create };
})();
