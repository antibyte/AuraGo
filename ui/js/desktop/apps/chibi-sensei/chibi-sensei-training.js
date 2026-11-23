// Chibi Sensei training step: the preflight list (grouped requirements, block and note styling, refresh, the admin dialog that
// switches the missing Kaggle settings on by flag name, the text that tells others what to ask for), the preset cards, the
// advanced overrides, the license acceptance and the start: prepare, a confirmation built from the server's answer, then the
// start with the plan token (plan_expired prepares again in the same dialog). The run monitor is P.run (chibi-sensei-run.js),
// mounted when present. Also the "comes later" state of the Test and Use steps. Pure helpers are exported for the Node test.
// Classic script, publishes P.training.
(function () {
    'use strict';

    const P = window.ChibiSenseiParts = window.ChibiSenseiParts || {};
    const PRESETS = ['quick', 'balanced', 'thorough'];
    const PRESET_KEYS = {
        quick: { name: 'chibi_sensei.training.preset_quick', desc: 'chibi_sensei.training.preset_quick_desc' },
        balanced: { name: 'chibi_sensei.training.preset_balanced', desc: 'chibi_sensei.training.preset_balanced_desc' },
        thorough: { name: 'chibi_sensei.training.preset_thorough', desc: 'chibi_sensei.training.preset_thorough_desc' }
    };
    const LATER_KEYS = { test: 'chibi_sensei.later.test_text', use: 'chibi_sensei.later.use_text' };
    const REFRESH_MS = 1000;
    const PREFLIGHT_REFRESH_MS = 5000;
    const GROUPS = ['data', 'kaggle', 'account', 'quota', 'storage', 'license'];
    const GROUP_KEYS = {
        data: 'chibi_sensei.preflight.group_data', kaggle: 'chibi_sensei.preflight.group_kaggle', account: 'chibi_sensei.preflight.group_account',
        quota: 'chibi_sensei.preflight.group_quota', storage: 'chibi_sensei.preflight.group_storage', license: 'chibi_sensei.preflight.group_license',
        other: 'chibi_sensei.preflight.group_other'
    };
    const LEVEL_KEYS = { block: 'chibi_sensei.preflight.level_block', warn: 'chibi_sensei.preflight.level_warn' };
    // label: the requirement in words (also used in the enable dialog); fix: what to do when it fails ('' = flag text or none)
    const ITEM_KEYS = {
        dataset_ready: { label: 'chibi_sensei.preflight.dataset_ready', fix: 'chibi_sensei.preflight.dataset_ready_fix' },
        run_active: { label: 'chibi_sensei.preflight.run_active', fix: 'chibi_sensei.preflight.run_active_fix' },
        kaggle_enabled: { label: 'chibi_sensei.preflight.kaggle_enabled', fix: '' },
        kaggle_not_readonly: { label: 'chibi_sensei.preflight.kaggle_not_readonly', fix: '' },
        kaggle_allow_create: { label: 'chibi_sensei.preflight.kaggle_allow_create', fix: '' },
        kaggle_allow_remote_execute: { label: 'chibi_sensei.preflight.kaggle_allow_remote_execute', fix: '' },
        kaggle_allow_gpu: { label: 'chibi_sensei.preflight.kaggle_allow_gpu', fix: '' },
        kaggle_allow_internet: { label: 'chibi_sensei.preflight.kaggle_allow_internet', fix: '' },
        kaggle_allow_download: { label: 'chibi_sensei.preflight.kaggle_allow_download', fix: '' },
        kaggle_machine_shape: { label: 'chibi_sensei.preflight.kaggle_machine_shape', fix: '' },
        kaggle_allow_delete: { label: 'chibi_sensei.preflight.kaggle_allow_delete', fix: 'chibi_sensei.preflight.kaggle_allow_delete_fix' },
        network_access: { label: 'chibi_sensei.preflight.network_access', fix: 'chibi_sensei.preflight.network_access_fix' },
        filesystem_write: { label: 'chibi_sensei.preflight.filesystem_write', fix: 'chibi_sensei.preflight.filesystem_write_fix' },
        kaggle_vault: { label: 'chibi_sensei.preflight.kaggle_vault', fix: 'chibi_sensei.preflight.kaggle_vault_fix' },
        kaggle_credentials: { label: 'chibi_sensei.preflight.kaggle_credentials', fix: 'chibi_sensei.preflight.kaggle_credentials_fix' },
        kaggle_connection: { label: 'chibi_sensei.preflight.kaggle_connection', fix: 'chibi_sensei.preflight.kaggle_connection_fix' },
        kaggle_timeout: { label: 'chibi_sensei.preflight.kaggle_timeout', fix: 'chibi_sensei.preflight.kaggle_timeout_fix' },
        kaggle_quota: { label: 'chibi_sensei.preflight.kaggle_quota', fix: 'chibi_sensei.preflight.kaggle_quota_fix' },
        kaggle_slot: { label: 'chibi_sensei.preflight.kaggle_slot', fix: 'chibi_sensei.preflight.kaggle_slot_fix' },
        kaggle_transfer: { label: 'chibi_sensei.preflight.kaggle_transfer', fix: 'chibi_sensei.preflight.kaggle_transfer_fix' },
        disk_space: { label: 'chibi_sensei.preflight.disk_space', fix: 'chibi_sensei.preflight.disk_space_fix' },
        license_consent: { label: 'chibi_sensei.preflight.license_consent', fix: 'chibi_sensei.preflight.license_consent_fix' }
    };
    const FLAG_ITEMS = {
        'kaggle.enabled': 'kaggle_enabled', 'kaggle.read_only': 'kaggle_not_readonly', 'kaggle.allow_create': 'kaggle_allow_create',
        'kaggle.allow_remote_execute': 'kaggle_allow_remote_execute', 'kaggle.allow_gpu': 'kaggle_allow_gpu', 'kaggle.allow_internet': 'kaggle_allow_internet',
        'kaggle.allow_download': 'kaggle_allow_download', 'kaggle.allowed_machine_shapes': 'kaggle_machine_shape', 'kaggle.allow_delete': 'kaggle_allow_delete'
    };
    const SHAPE_KEYS = { NvidiaTeslaT4: 'chibi_sensei.shape.NvidiaTeslaT4', NvidiaL4: 'chibi_sensei.shape.NvidiaL4' };
    const METHOD_KEYS = { lora: 'chibi_sensei.training.method_lora', full: 'chibi_sensei.training.method_full' };
    const SEQ_BUCKETS = [256, 512, 1024, 2048, 4096];
    const OVERRIDE_FIELDS = ['method', 'epochs', 'learning_rate', 'max_seq_len', 'lora_r', 'lora_alpha'];
    const BAD_KEYS = {
        method: 'chibi_sensei.overrides.bad_method', epochs: 'chibi_sensei.overrides.bad_epochs', learning_rate: 'chibi_sensei.overrides.bad_lr',
        max_seq_len: 'chibi_sensei.overrides.bad_seq', lora_r: 'chibi_sensei.overrides.bad_lora_r', lora_alpha: 'chibi_sensei.overrides.bad_lora_alpha'
    };

    function lrText(value) {
        const v = Number(value);
        if (!Number.isFinite(v)) return '0';
        try { return new Intl.NumberFormat(String((typeof window !== 'undefined' && window.SYSTEM_LANG) || 'en'), { maximumSignificantDigits: 2 }).format(v); } catch (_) { return String(v); }
    }

    // ---- pure helpers (no DOM) ----------------------------------------------------------------------------------------------------

    // groupPreflight orders the items by group (GROUPS order, unknown groups last as "other") and counts what blocks.
    function groupPreflight(pf) {
        const items = pf && Array.isArray(pf.items) ? pf.items.filter(i => i && typeof i.id === 'string') : [];
        const byId = {}, groups = [];
        items.forEach(item => {
            const id = GROUPS.indexOf(item.group) >= 0 ? item.group : 'other';
            if (!byId[id]) { byId[id] = { id, items: [], failing: 0 }; groups.push(byId[id]); }
            byId[id].items.push(item);
            if (!item.ok) byId[id].failing++;
        });
        const rank = id => (GROUPS.indexOf(id) >= 0 ? GROUPS.indexOf(id) : GROUPS.length);
        groups.sort((a, b) => rank(a.id) - rank(b.id));
        const blocks = items.filter(i => !i.ok && i.level !== 'warn').length;
        const warns = items.filter(i => !i.ok && i.level === 'warn').length;
        return { groups, blocks, warns, ready: !!(pf && pf.ready) && blocks === 0 };
    }

    // The flags the enable dialog offers, from the server's fix list (deduped). Flags of blocking items are required and always
    // applied; a flag of a note (allow_delete) is optional and can be unticked. Required come first.
    function enableFlags(pf) {
        const fix = pf && pf.fix && Array.isArray(pf.fix.flags) ? pf.fix.flags : [];
        const items = pf && Array.isArray(pf.items) ? pf.items : [];
        const seen = new Set(), out = [];
        fix.forEach(flag => {
            if (typeof flag !== 'string' || !flag || seen.has(flag)) return;
            seen.add(flag);
            const item = items.find(i => i && i.flag === flag && !i.ok);
            out.push({ flag, id: item ? item.id : (FLAG_ITEMS[flag] || ''), optional: !!item && item.level === 'warn' });
        });
        return out.sort((a, b) => Number(a.optional) - Number(b.optional));
    }

    function profileMethods(profile) {
        const p = profile || {};
        return Array.isArray(p.methods) && p.methods.length ? p.methods.slice() : [p.method_default || 'lora'];
    }
    function seqCap(profile) {
        const p = profile || {};
        return Number(p.max_seq_cap) > 0 ? Number(p.max_seq_cap) : Math.min(4096, Number(p.context_max) || 4096);
    }
    function seqOptions(profile) { const cap = seqCap(profile); return SEQ_BUCKETS.filter(b => b <= cap); }

    // validateOverrides mirrors chibisensei.ValidateOverrides. draft holds the form's strings ('' = automatic). It returns only the
    // values that are set; LoRA fields count only when the effective method is LoRA.
    function validateOverrides(draft, profile) {
        const d = draft || {}, out = {}, issues = [];
        const raw = name => String(d[name] == null ? '' : d[name]).trim();
        const int = (name, min, max) => {
            const s = raw(name);
            if (!s) return;
            const n = Number(s);
            if (!/^\d+$/.test(s) || n < min || n > max) issues.push({ field: name, key: BAD_KEYS[name] }); else out[name] = n;
        };
        const method = raw('method');
        if (method) { if (profileMethods(profile).indexOf(method) >= 0) out.method = method; else issues.push({ field: 'method', key: BAD_KEYS.method }); }
        int('epochs', 1, 10);
        const lr = raw('learning_rate').replace(',', '.');
        if (lr) {
            const n = Number(lr);
            if (!Number.isFinite(n) || n < 1e-6 || n > 1e-3) issues.push({ field: 'learning_rate', key: BAD_KEYS.learning_rate }); else out.learning_rate = n;
        }
        const seq = raw('max_seq_len');
        if (seq) { const n = Number(seq); if (seqOptions(profile).indexOf(n) < 0) issues.push({ field: 'max_seq_len', key: BAD_KEYS.max_seq_len }); else out.max_seq_len = n; }
        if ((out.method || (profile && profile.method_default) || 'lora') === 'lora') { int('lora_r', 1, 256); int('lora_alpha', 1, 512); }
        return { overrides: out, issues };
    }

    function shapeText(t, shape) { return SHAPE_KEYS[shape] ? t(SHAPE_KEYS[shape]) : String(shape || ''); }
    function trainingShape(inst) { return (inst.meta && inst.meta.training && inst.meta.training.machine_shape) || 'NvidiaTeslaT4'; }
    // Placeholders of an item text. A missing arg shows a dash, so a text never carries a raw {name}.
    function itemVars(inst, item) {
        const vars = { shape: shapeText(inst.t, trainingShape(inst)), minutes: '-', needed: '-', jobs: '-' };
        const args = item && item.args && typeof item.args === 'object' ? item.args : {};
        Object.keys(args).forEach(k => {
            const v = args[k];
            if (v == null) return;
            vars[k] = typeof v === 'number' ? P.format.number(v) : (Array.isArray(v) ? v.map(String).join(', ') : String(v));
        });
        return vars;
    }
    function itemLabel(t, item, vars) {
        const keys = ITEM_KEYS[item.id];
        return keys ? t(keys.label, vars) : P.errors.knownKey(t, item.message_key, 'chibi_sensei.preflight.unknown', vars);
    }
    function fixText(t, item, vars, admin) {
        if (item.id === 'kaggle_quota' && item.level === 'warn') return t('chibi_sensei.preflight.kaggle_quota_warn_fix', vars);
        const keys = ITEM_KEYS[item.id];
        if (keys && keys.fix) return t(keys.fix, vars);
        if (item.flag) return t(admin ? 'chibi_sensei.preflight.flag_fix_admin' : 'chibi_sensei.preflight.flag_fix_ask');
        return '';
    }
    function flagLabel(inst, f) {
        const id = f.id || FLAG_ITEMS[f.flag] || '';
        return itemLabel(inst.t, { id, message_key: '' }, itemVars(inst, null));
    }
    function profileOf(inst) {
        const profiles = inst.meta && Array.isArray(inst.meta.profiles) ? inst.meta.profiles : [];
        const id = inst.project ? inst.project.profile_id : '';
        return profiles.find(p => p && p.id === id) || null;
    }

    // ---- preflight panel ----------------------------------------------------------------------------------------------------------

    function createPreflight(inst, hooks) {
        const { el, icon } = P;
        const t = inst.t;
        const checked = el('span', { class: 'csn-hint', dataset: { csn: 'preflight-checked' } });
        const refresh = el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'preflight-refresh' }, onClick: () => hooks.reload() },
            icon('refresh'), el('span', { text: t('chibi_sensei.preflight.refresh') }));
        const summary = el('p', { class: 'csn-preflight-summary', role: 'status', dataset: { csn: 'preflight-summary', state: 'loading' }, text: t('chibi_sensei.preflight.checking') });
        const error = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'preflight-error' } });
        error.hidden = true;
        const groupsBox = el('div', { class: 'csn-pf-groups' });
        const fixBox = el('div', { class: 'csn-pf-fixbar', dataset: { csn: 'preflight-fix' } });
        fixBox.hidden = true;
        const node = el('section', { class: 'csn-training-section csn-preflight', dataset: { csn: 'preflight' }, ariaLabel: t('chibi_sensei.preflight.title') },
            el('div', { class: 'csn-section-head' }, el('h3', { text: t('chibi_sensei.preflight.title') }), checked, refresh),
            el('p', { class: 'csn-muted', text: t('chibi_sensei.preflight.intro') }), summary, error, groupsBox, fixBox);
        let last = null;

        function setLoading(on) {
            node.setAttribute('aria-busy', on ? 'true' : 'false');
            refresh.disabled = !!on;
            if (on && !last) { summary.dataset.state = 'loading'; summary.textContent = t('chibi_sensei.preflight.checking'); }
        }

        function itemNode(it, admin) {
            const level = it.level === 'warn' ? 'warn' : 'block';
            const vars = itemVars(inst, it);
            const li = el('li', { class: 'csn-pf-item', dataset: { csn: 'preflight-item', id: it.id, level, ok: it.ok ? '1' : '0' } },
                icon(it.ok ? 'check' : (level === 'warn' ? 'info' : 'alert')),
                el('span', { class: 'csn-pf-text', text: itemLabel(t, it, vars) }),
                el('span', { class: 'csn-pf-state', text: t(it.ok ? 'chibi_sensei.preflight.level_ok' : LEVEL_KEYS[level]) }));
            if (!it.ok) {
                const fix = fixText(t, it, vars, admin);
                if (fix) li.appendChild(el('p', { class: 'csn-hint csn-pf-fix', text: fix }));
                if (it.link && P.safeLink(it.link)) li.appendChild(el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'preflight-link' }, text: t('chibi_sensei.preflight.open_link'), onClick: () => P.openLink(it.link) }));
            }
            return li;
        }

        function paintFix(pf, admin) {
            P.clear(fixBox);
            const flags = enableFlags(pf);
            fixBox.hidden = !flags.length;
            if (!flags.length) return;
            if (admin) {
                const button = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'preflight-enable' }, onClick: () => hooks.enable(pf) },
                    icon('check'), el('span', { text: t('chibi_sensei.enable.button') }));
                button.disabled = inst.readonly;
                button.title = inst.readonly ? t('chibi_sensei.readonly_banner') : '';
                fixBox.appendChild(el('p', { class: 'csn-hint', text: t('chibi_sensei.enable.lead_admin') }));
                fixBox.appendChild(button);
                return;
            }
            fixBox.appendChild(el('div', { class: 'csn-note', dataset: { csn: 'preflight-ask' } },
                el('strong', { text: t('chibi_sensei.enable.ask_title') }), el('p', { text: t('chibi_sensei.enable.ask_text') }),
                el('ul', { class: 'csn-flag-list' }, flags.map(f => el('li', { text: flagLabel(inst, f) })))));
        }

        function paint(pf) {
            last = pf;
            error.hidden = true;
            const g = groupPreflight(pf);
            const admin = !!(pf && pf.admin);
            P.clear(groupsBox);
            g.groups.forEach(group => groupsBox.appendChild(el('section', { class: 'csn-pf-group', dataset: { csn: 'preflight-group', group: group.id, failing: String(group.failing) } },
                el('h4', { class: 'csn-pf-group-title', text: t(GROUP_KEYS[group.id] || GROUP_KEYS.other) }),
                group.id === 'quota' ? el('p', { class: 'csn-hint', text: t('chibi_sensei.preflight.quota_help') }) : null,
                el('ul', { class: 'csn-pf-list' }, group.items.map(it => itemNode(it, admin))))));
            summary.dataset.state = g.blocks ? 'blocked' : (g.warns ? 'warn' : 'ready');
            summary.textContent = g.blocks ? t('chibi_sensei.preflight.summary_blocked', { count: P.format.number(g.blocks) })
                : (g.warns ? t('chibi_sensei.preflight.summary_warn', { count: P.format.number(g.warns) }) : t('chibi_sensei.preflight.summary_ready'));
            checked.textContent = pf && pf.checked_at ? t('chibi_sensei.preflight.checked_at', { time: P.format.dateTime(pf.checked_at) }) : '';
            paintFix(pf, admin);
        }

        function fail(text) {
            error.textContent = text || '';
            error.hidden = !text;
            summary.dataset.state = 'error';
            summary.textContent = t('chibi_sensei.preflight.failed');
        }

        return { el: node, paint, setLoading, fail, repaint() { if (last) paint(last); } };
    }

    // ---- enable dialog ---------------------------------------------------------------------------------------------------------------

    function openEnable(inst, pf, done) {
        const { el } = P;
        const t = inst.t;
        const flags = enableFlags(pf);
        if (!flags.length || inst.readonly || !P.table) return;
        const dlg = P.table.makeDialog(inst, t('chibi_sensei.enable.title'), 'enable-dialog', false);
        const required = flags.filter(f => !f.optional), optional = flags.filter(f => f.optional);
        const boxes = optional.map(f => el('input', { type: 'checkbox', checked: true, dataset: { csn: 'enable-flag', flag: f.flag } }));
        const err = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'enable-error' } });
        err.hidden = true;
        const cancel = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'enable-cancel' }, text: t('chibi_sensei.enable.cancel'), onClick: () => dlg.close() });
        const ok = el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'enable-confirm' }, text: t('chibi_sensei.enable.confirm') });
        dlg.content.appendChild(el('p', { text: t('chibi_sensei.enable.intro') }));
        if (required.length) dlg.content.appendChild(el('div', {}, el('h4', { class: 'csn-subhead', text: t('chibi_sensei.enable.required') }),
            el('ul', { class: 'csn-flag-list', dataset: { csn: 'enable-required' } }, required.map(f => el('li', { dataset: { flag: f.flag }, text: flagLabel(inst, f) })))));
        if (optional.length) dlg.content.appendChild(el('div', { class: 'csn-checkgroup', dataset: { csn: 'enable-optional' } },
            el('h4', { class: 'csn-subhead', text: t('chibi_sensei.enable.optional') }),
            optional.map((f, i) => el('label', { class: 'csn-check-option' }, boxes[i], el('span', { text: flagLabel(inst, f) })))));
        dlg.content.appendChild(err);
        dlg.content.appendChild(el('div', { class: 'csn-actions csn-dialog-actions' }, cancel, ok));
        let busy = false;
        ok.addEventListener('click', async () => {
            if (busy) return;
            const chosen = required.map(f => f.flag).concat(boxes.filter(b => b.checked).map(b => b.dataset.flag));
            if (!chosen.length) { err.textContent = t('chibi_sensei.enable.none_selected'); err.hidden = false; return; }
            busy = true; ok.disabled = true; cancel.disabled = true; ok.textContent = t('chibi_sensei.pending'); err.hidden = true;
            try {
                const res = await inst.api.enablePreflight(chosen);
                if (inst.disposed) return;
                dlg.close();
                const n = Array.isArray(res && res.applied) ? res.applied.length : chosen.length;
                inst.notify(t('chibi_sensei.enable.applied', { count: P.format.number(n) }), 'info');
                done();
            } catch (e) {
                if (inst.disposed || (e && e.aborted)) return;
                err.textContent = inst.handleError(e); err.hidden = false;
            } finally {
                busy = false;
                if (!inst.disposed) { ok.disabled = false; cancel.disabled = false; ok.textContent = t('chibi_sensei.enable.confirm'); }
            }
        });
        dlg.open();
        ok.focus();
    }

    // ---- license acceptance ----------------------------------------------------------------------------------------------------------

    function createLicense(inst, hooks) {
        const { el } = P;
        const t = inst.t;
        const node = el('section', { class: 'csn-training-section csn-license', dataset: { csn: 'license' } });
        node.hidden = true;
        let busy = false;

        function paint(profile) {
            P.clear(node);
            const lic = profile && profile.license && profile.license.requires_consent ? profile.license : null;
            node.hidden = !lic;
            if (!lic) return;
            const vars = { model: profile.display_name || profile.id, license: lic.name || '' };
            node.appendChild(el('h3', { text: t('chibi_sensei.consent.title') }));
            node.appendChild(el('p', { text: t('chibi_sensei.consent.text', vars) }));
            node.appendChild(el('span', { class: 'csn-label', text: t('chibi_sensei.consent.link_label') }));
            node.appendChild(P.linkRow(t, lic.url, 'license-link'));
            if (lic.accepted) {
                node.appendChild(el('p', { class: 'csn-ok-line', dataset: { csn: 'license-accepted' }, text: t('chibi_sensei.consent.accepted', vars) }));
                return;
            }
            const check = el('input', { type: 'checkbox', dataset: { csn: 'license-check' } });
            const accept = el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'license-accept' }, text: t('chibi_sensei.consent.accept') });
            const err = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'license-error' } });
            err.hidden = true;
            const sync = () => {
                accept.disabled = !check.checked || inst.readonly || busy;
                accept.title = inst.readonly ? t('chibi_sensei.readonly_banner') : '';
            };
            check.addEventListener('change', sync);
            accept.addEventListener('click', async () => {
                if (busy || inst.readonly || !check.checked) return;
                busy = true; err.hidden = true; sync();
                accept.textContent = t('chibi_sensei.pending');
                try {
                    await inst.api.consent(profile.id);
                    if (inst.disposed) return;
                    lic.accepted = true;
                    inst.notify(t('chibi_sensei.consent.saved'), 'info');
                    busy = false;
                    paint(profile);
                    hooks.accepted();
                } catch (e) {
                    if (inst.disposed || (e && e.aborted)) return;
                    err.textContent = inst.handleError(e); err.hidden = false;
                } finally {
                    busy = false;
                    if (!inst.disposed && accept.isConnected) { accept.textContent = t('chibi_sensei.consent.accept'); sync(); }
                }
            });
            sync();
            node.appendChild(el('label', { class: 'csn-check-option' }, check, el('span', { text: t('chibi_sensei.consent.accept_label', vars) })));
            node.appendChild(accept);
            node.appendChild(err);
        }

        return { el: node, paint };
    }

    // ---- advanced overrides ----------------------------------------------------------------------------------------------------------

    function createOverrides(inst, hooks) {
        const { el } = P;
        const t = inst.t;
        const num = P.format.number;
        const grid = el('div', { class: 'csn-grid2' });
        const formError = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'overrides-error' } });
        formError.hidden = true;
        const saveButton = el('button', { type: 'submit', class: 'csn-btn csn-btn-primary', dataset: { csn: 'overrides-save' }, text: t('chibi_sensei.overrides.save') });
        const resetButton = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'overrides-reset' }, text: t('chibi_sensei.overrides.reset'), onClick: () => save(true) });
        const form = el('form', { class: 'csn-overrides', novalidate: true, dataset: { csn: 'overrides' } },
            grid, formError, el('div', { class: 'csn-actions' }, saveButton, resetButton));
        const node = el('details', { class: 'csn-advanced', dataset: { csn: 'training-advanced' } },
            el('summary', { text: t('chibi_sensei.training.advanced') }),
            el('p', { class: 'csn-muted', text: t('chibi_sensei.training.advanced_text') }),
            form);
        let fields = {};
        let builtKey = '';
        let dirty = false;
        let saving = false;
        let plan = null;

        function field(name, labelKey, helpKey, control) {
            const id = 'csn-' + inst.windowId + '-ov-' + name;
            const err = el('p', { class: 'csn-error', dataset: { csn: 'ov-' + name + '-error' } });
            err.id = id + '-error';
            err.hidden = true;
            control.id = id;
            control.dataset.csn = 'ov-' + name;
            control.setAttribute('aria-describedby', err.id);
            const touched = () => { dirty = true; updateLora(); };
            control.addEventListener('input', touched);
            control.addEventListener('change', touched);
            const wrap = el('label', { class: 'csn-field', dataset: { field: name } }, el('span', { class: 'csn-label', text: t(labelKey) }), control, helpKey ? el('span', { class: 'csn-hint', text: t(helpKey) }) : null, err);
            fields[name] = { wrap, control, err };
            grid.appendChild(wrap);
        }
        const numberInput = (min, max) => el('input', { class: 'csn-input', type: 'number', min, max, step: 1 });

        function build(profile) {
            const methods = profileMethods(profile), seqs = seqOptions(profile);
            const key = [profile ? profile.id : '', methods.join(','), seqs.join(',')].join('|');
            if (key === builtKey) return;
            builtKey = key;
            fields = {};
            P.clear(grid);
            dirty = false;
            const fallback = (profile && profile.method_default) || methods[0] || 'lora';
            if (methods.length > 1) {
                const select = el('select', { class: 'csn-select' },
                    el('option', { value: '', text: t('chibi_sensei.overrides.method_auto', { value: t(METHOD_KEYS[fallback] || METHOD_KEYS.lora) }) }),
                    methods.map(m => el('option', { value: m, text: t(METHOD_KEYS[m] || METHOD_KEYS.lora) })));
                field('method', 'chibi_sensei.overrides.method', '', select);
            }
            field('epochs', 'chibi_sensei.overrides.epochs', 'chibi_sensei.overrides.epochs_help', numberInput(1, 10));
            field('learning_rate', 'chibi_sensei.overrides.lr', 'chibi_sensei.overrides.lr_help', el('input', { class: 'csn-input', type: 'text', inputmode: 'decimal' }));
            field('max_seq_len', 'chibi_sensei.overrides.seq', 'chibi_sensei.overrides.seq_help', el('select', { class: 'csn-select' },
                el('option', { value: '', text: t('chibi_sensei.overrides.auto_option') }), seqs.map(n => el('option', { value: String(n), text: num(n) }))));
            field('lora_r', 'chibi_sensei.overrides.lora_r', 'chibi_sensei.overrides.lora_r_help', numberInput(1, 256));
            field('lora_alpha', 'chibi_sensei.overrides.lora_alpha', 'chibi_sensei.overrides.lora_alpha_help', numberInput(1, 512));
        }

        function effectiveMethod() {
            const profile = hooks.profile();
            const chosen = fields.method ? fields.method.control.value : '';
            return chosen || (profile && profile.method_default) || 'lora';
        }
        function updateLora() {
            const show = effectiveMethod() !== 'full';
            ['lora_r', 'lora_alpha'].forEach(name => { if (fields[name]) fields[name].wrap.hidden = !show; });
        }

        function fill() {
            if (dirty) return;
            const ov = (inst.project && inst.project.overrides) || {};
            OVERRIDE_FIELDS.forEach(name => { const f = fields[name]; if (f) f.control.value = ov[name] == null ? '' : String(ov[name]); });
        }

        function placeholders() {
            const h = (plan && plan.hyper) || {};
            const auto = value => t('chibi_sensei.overrides.auto', { value });
            if (fields.epochs && h.epochs != null) fields.epochs.control.placeholder = auto(num(h.epochs));
            if (fields.learning_rate && h.learning_rate != null) fields.learning_rate.control.placeholder = auto(lrText(h.learning_rate));
            if (fields.lora_r && h.lora && h.lora.r != null) fields.lora_r.control.placeholder = auto(num(h.lora.r));
            if (fields.lora_alpha && h.lora && h.lora.alpha != null) fields.lora_alpha.control.placeholder = auto(num(h.lora.alpha));
        }

        function setDisabled() {
            const off = inst.readonly || saving;
            OVERRIDE_FIELDS.forEach(name => { if (fields[name]) fields[name].control.disabled = off; });
            saveButton.disabled = off;
            resetButton.disabled = off;
            saveButton.title = inst.readonly ? t('chibi_sensei.readonly_banner') : '';
        }

        function readDraft() {
            const out = {};
            OVERRIDE_FIELDS.forEach(name => { out[name] = fields[name] ? fields[name].control.value : ''; });
            return out;
        }

        function clearIssues() {
            OVERRIDE_FIELDS.forEach(name => {
                const f = fields[name];
                if (!f) return;
                f.control.removeAttribute('aria-invalid');
                f.err.textContent = '';
                f.err.hidden = true;
            });
            formError.textContent = '';
            formError.hidden = true;
        }

        function showIssues(list) {
            let first = null;
            list.forEach(issue => {
                const f = fields[issue.field];
                if (!f) return;
                f.control.setAttribute('aria-invalid', 'true');
                f.err.textContent = t(issue.key);
                f.err.hidden = false;
                if (!first) first = f.control;
            });
            formError.textContent = t('chibi_sensei.overrides.fix_fields');
            formError.hidden = false;
            if (first) first.focus();
        }

        function showFormError(text) { formError.textContent = text || ''; formError.hidden = !text; }

        async function save(reset) {
            if (saving || inst.readonly || !inst.project) return;
            clearIssues();
            let next = {};
            if (!reset) {
                const result = validateOverrides(readDraft(), hooks.profile());
                if (result.issues.length) { showIssues(result.issues); return; }
                next = result.overrides;
            }
            saving = true; setDisabled();
            const mine = inst.projectSeq;
            try {
                const updated = await inst.api.patchProject(inst.project.id, inst.project.rev, { overrides: next });
                if (mine !== inst.projectSeq || inst.disposed) return;
                dirty = false;
                hooks.saved(updated);
                fill();
                inst.notify(t('chibi_sensei.overrides.saved'), 'info');
            } catch (err) {
                if (inst.disposed || (err && err.aborted)) return;
                const e = P.errors.normalize(err);
                const names = e.issues.map(i => String(i && i.field || '').replace(/^overrides\./, '')).filter(f => BAD_KEYS[f]);
                if (e.status === 422 && names.length) showIssues(names.map(field => ({ field, key: BAD_KEYS[field] })));
                else showFormError(inst.handleError(err));
            } finally { saving = false; if (!inst.disposed) setDisabled(); }
        }

        form.addEventListener('submit', event => { event.preventDefault(); save(false); });

        // paint rebuilds the controls only when the base model changes; values are refilled while the form is not dirty.
        function paint(selectedPlan) {
            plan = selectedPlan || null;
            build(hooks.profile());
            placeholders();
            fill();
            updateLora();
            setDisabled();
        }

        return { el: node, paint };
    }

    // ---- confirmation text ---------------------------------------------------------------------------------------------------------

    function confirmDetails(inst, prep) {
        const { el } = P;
        const t = inst.t, num = P.format.number;
        const d = prep || {}, data = d.data || {}, rows = data.rows || {}, dest = d.destinations || {};
        const lic = d.license || {}, time = d.time || {}, clean = d.cleanup || {};
        const p = (text, name) => el('p', { dataset: name ? { csn: name } : null, text });
        const block = (name, title, lines) => el('section', { class: 'csn-confirm-block', dataset: { csn: 'confirm-' + name } }, el('h4', { text: title }), lines);
        const session = P.format.minutes(time.session_timeout_minutes);
        const hours = time.quota_hours_remaining;
        const afterKey = !clean.delete_remote_after_run ? 'chibi_sensei.confirm.after_keep_setting'
            : (clean.allow_delete ? 'chibi_sensei.confirm.after_delete' : 'chibi_sensei.confirm.after_keep_policy');
        return el('div', { class: 'csn-confirm' },
            block('data', t('chibi_sensei.confirm.data_title'), [
                p(t('chibi_sensei.confirm.data_rows', { train: num(rows.train), val: num(rows.val), eval: num(rows.eval), size: P.format.bytes(data.bytes) }), 'confirm-rows'),
                p(t('chibi_sensei.confirm.data_private')),
                p(t('chibi_sensei.confirm.dataset', { name: String(dest.dataset || '') }), 'confirm-dataset'),
                p(t('chibi_sensei.confirm.notebook', { name: String(dest.notebook || '') }), 'confirm-notebook'),
                p(t('chibi_sensei.confirm.notebook_help')),
                p(t('chibi_sensei.training.gpu', { shape: shapeText(t, dest.machine_shape) }), 'confirm-gpu')]),
            block('license', t('chibi_sensei.confirm.license', { name: String(lic.name || lic.id || '') }), [lic.url ? P.linkRow(t, lic.url, 'confirm-license-link') : null]),
            block('time', t('chibi_sensei.confirm.time_title'), [
                p(t('chibi_sensei.confirm.estimate', { minutes: P.format.minutes(time.estimate_minutes) }), 'confirm-estimate'),
                p(t('chibi_sensei.confirm.session', { minutes: session }), 'confirm-session'),
                p(typeof hours === 'number' && Number.isFinite(hours) ? t('chibi_sensei.confirm.quota', { hours: P.format.decimal(hours, 1) }) : t('chibi_sensei.confirm.quota_unknown'), 'confirm-quota')]),
            block('stop', t('chibi_sensei.confirm.cancel_title'), [p(t('chibi_sensei.confirm.cancel_text', { minutes: session }), 'confirm-cancel-text')]),
            block('after', t('chibi_sensei.confirm.after_title'), [p(t(afterKey), 'confirm-after')]),
            p(t('chibi_sensei.confirm.valid', { minutes: P.format.minutes((Number(d.expires_in_s) || 900) / 60) }), 'confirm-valid'));
    }

    // ---- the step -----------------------------------------------------------------------------------------------------------------

    function create(inst) {
        const { el } = P;
        const t = inst.t;
        const num = P.format.number;
        inst.preflight = null;
        const node = el('section', { class: 'csn-training', dataset: { csn: 'training' } });
        const error = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'training-error' } });
        error.hidden = true;
        const top = el('div', { class: 'csn-training-body', dataset: { csn: 'training-top' } });
        const body = el('div', { class: 'csn-training-body' });
        const bottom = el('div', { class: 'csn-training-body', dataset: { csn: 'training-bottom' } });
        node.appendChild(el('h2', { class: 'csn-step-title', text: t('chibi_sensei.training.title') }));
        node.appendChild(el('p', { class: 'csn-muted', text: t('chibi_sensei.training.intro') }));
        node.appendChild(error);
        node.appendChild(top);
        node.appendChild(body);
        node.appendChild(bottom);
        let disposed = false;
        let plans = null;
        let seq = 0;
        let pfSeq = 0;
        let saving = false;
        let loadedRev = null;
        let mode = '';

        const showError = text => { error.textContent = text || ''; error.hidden = !text; };
        const currentPreset = () => (inst.project && PRESETS.indexOf(inst.project.preset) >= 0 ? inst.project.preset : 'balanced');
        const selectedPlan = () => (plans ? plans[PRESETS.indexOf(currentPreset())] || null : null);

        const runArea = P.run ? P.run.createArea(inst) : null;

        // ---- sections ----
        const pf = createPreflight(inst, { reload: () => loadPreflight(), enable: pfData => openEnable(inst, pfData, loadPreflight) });
        const lic = createLicense(inst, { accepted: () => loadPreflight() });
        const overrides = createOverrides(inst, {
            profile: () => profileOf(inst),
            saved: updated => { inst.project = updated; inst.bus.emit('project', updated); inst.recompute(); }
        });
        const grid = el('div', { class: 'csn-preset-grid', role: 'group', ariaLabel: t('chibi_sensei.training.title'), dataset: { csn: 'preset-grid' } });
        const note = el('div', { class: 'csn-note', dataset: { csn: 'plan-note' } });
        note.hidden = true;
        const gpu = el('p', { class: 'csn-hint', dataset: { csn: 'plan-gpu' } });
        const presets = el('section', { class: 'csn-training-section', dataset: { csn: 'presets' } },
            el('h3', { text: t('chibi_sensei.training.preset_title') }),
            el('p', { class: 'csn-muted', text: t('chibi_sensei.training.preset_hint') }),
            gpu, grid,
            el('p', { class: 'csn-hint', text: t('chibi_sensei.training.timeout_help') }),
            note);

        function empty(key, withGoal) {
            P.clear(body);
            mode = 'empty';
            const card = el('div', { class: 'csn-state csn-state-inline', role: 'status', dataset: { csn: 'training-empty' } }, el('p', { text: t(key) }));
            if (withGoal) card.appendChild(el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'training-to-goal' }, text: t('chibi_sensei.training.to_goal'), onClick: () => inst.showStep('goal') }));
            body.appendChild(card);
        }

        // true when the sections are shown; false while the project has no base model or no rows
        function layout() {
            const p = inst.project;
            if (!p) return false;
            if (!p.profile_id) { empty('chibi_sensei.training.no_profile', true); return false; }
            if (!(Number(p.rows) > 0)) { empty('chibi_sensei.training.no_data', false); return false; }
            if (mode !== 'full') {
                P.clear(body);
                body.appendChild(pf.el);
                body.appendChild(lic.el);
                body.appendChild(presets);
                body.appendChild(overrides.el);
                mode = 'full';
            }
            return true;
        }

        function arrange() {
            if (!runArea || !runArea.el) return;
            const target = P.activeRun(inst.runs) ? top : bottom;
            if (runArea.el.parentNode !== target) target.appendChild(runArea.el);
        }

        async function choose(preset) {
            if (inst.readonly || saving || !inst.project || inst.project.preset === preset) return;
            saving = true;
            showError('');
            const mine = inst.projectSeq;
            try {
                const updated = await inst.api.patchProject(inst.project.id, inst.project.rev, { preset });
                if (mine !== inst.projectSeq || inst.disposed) return;
                inst.project = updated;
                inst.bus.emit('project', updated);
                inst.recompute();
                if (disposed) return;
                inst.notify(t('chibi_sensei.saved'), 'info');
                loadPreflight();
            } catch (err) {
                if (disposed || (err && err.aborted)) return;
                showError(inst.handleError(err));
            } finally {
                saving = false;
                if (!disposed) paintAll();
            }
        }

        function line(text, name) { return el('li', { class: 'csn-plan-line', dataset: { csn: name }, text }); }

        function card(preset, plan) {
            const h = plan.hyper || {};
            const selected = !!inst.project && inst.project.preset === preset;
            const names = PRESET_KEYS[preset];
            const btn = el('button', { type: 'button', class: 'csn-preset-card', ariaPressed: selected ? 'true' : 'false', dataset: { csn: 'preset-card', preset }, onClick: () => choose(preset) });
            btn.disabled = inst.readonly;
            btn.title = inst.readonly ? t('chibi_sensei.readonly_banner') : '';
            const head = el('span', { class: 'csn-preset-head' }, el('strong', { text: t(names.name) }), selected ? el('span', { class: 'csn-badge', text: t('chibi_sensei.training.selected') }) : null);
            const list = el('ul', { class: 'csn-plan-list' });
            list.appendChild(line(t('chibi_sensei.training.estimate', { minutes: P.format.minutes(plan.estimate_minutes) }), 'plan-estimate'));
            list.appendChild(line(t('chibi_sensei.training.timeout', { minutes: P.format.minutes(plan.session_timeout_minutes) }), 'plan-timeout'));
            list.appendChild(line(t('chibi_sensei.training.epochs', { value: num(h.epochs) }), 'plan-epochs'));
            list.appendChild(line(t('chibi_sensei.training.lr', { value: lrText(h.learning_rate) }), 'plan-lr'));
            list.appendChild(line(t('chibi_sensei.training.batch', { batch: num(h.per_device_batch), accum: num(h.grad_accum) }), 'plan-batch'));
            list.appendChild(line(t('chibi_sensei.training.seq', { value: num(h.max_seq_len) }), 'plan-seq'));
            if (plan.method === 'full') list.appendChild(line(t('chibi_sensei.training.method_full'), 'plan-method'));
            else {
                list.appendChild(line(t('chibi_sensei.training.method_lora'), 'plan-method'));
                if (h.lora) list.appendChild(line(t('chibi_sensei.training.lora', { r: num(h.lora.r), alpha: num(h.lora.alpha), dropout: lrText(h.lora.dropout) }), 'plan-lora'));
            }
            const sp = plan.split || {};
            list.appendChild(line(t('chibi_sensei.training.split', { train: num(sp.train), val: num(sp.val), eval: num(sp.eval) }), 'plan-split'));
            if (Number(plan.eval_samples) > 0) list.appendChild(line(t('chibi_sensei.training.eval_samples', { count: num(plan.eval_samples) }), 'plan-eval'));
            if (Number(plan.artifacts_estimated_mb) > 0) list.appendChild(line(t('chibi_sensei.training.download_size', { mb: num(plan.artifacts_estimated_mb) }), 'plan-download'));
            btn.appendChild(head);
            btn.appendChild(el('span', { class: 'csn-muted', text: t(names.desc) }));
            btn.appendChild(list);
            return btn;
        }

        function paintPresets() {
            if (!plans) return;
            P.clear(grid);
            PRESETS.forEach((preset, i) => { if (plans[i]) grid.appendChild(card(preset, plans[i])); });
            gpu.textContent = t('chibi_sensei.training.gpu', { shape: shapeText(t, trainingShape(inst)) });
            const plan = selectedPlan();
            P.clear(note);
            note.hidden = !(plan && plan.trainable === false);
            if (!note.hidden) note.appendChild(el('span', { text: P.errors.knownKey(t, plan.reason_key, 'chibi_sensei.plan.training_later') }));
        }

        function paintAll() {
            if (disposed || mode !== 'full') return;
            paintPresets();
            overrides.paint(selectedPlan());
            lic.paint(profileOf(inst));
        }

        async function loadPreflight() {
            if (disposed || !inst.project || !inst.project.profile_id) return;
            const mine = ++pfSeq, id = inst.project.id;
            pf.setLoading(true);
            try {
                const result = await inst.api.preflight(id, currentPreset());
                if (disposed || mine !== pfSeq || !inst.project || inst.project.id !== id) return;
                inst.preflight = result;
                pf.paint(result);
            } catch (err) {
                if (disposed || mine !== pfSeq || (err && err.aborted)) return;
                inst.preflight = null;
                pf.fail(inst.handleError(err));
            } finally {
                if (!disposed && mine === pfSeq) { pf.setLoading(false); inst.recompute(); }
            }
        }

        async function refresh() {
            if (disposed || !inst.project) return;
            if (!layout()) { plans = null; pfSeq++; inst.preflight = null; inst.recompute(); return; }
            loadPreflight();
            const mine = ++seq;
            const id = inst.project.id;
            loadedRev = inst.project.data_rev;
            try {
                const result = await Promise.all(PRESETS.map(preset => inst.api.plan(id, preset)));
                if (disposed || mine !== seq) return;
                plans = result;
                showError('');
                paintAll();
            } catch (err) {
                if (disposed || mine !== seq || (err && err.aborted)) return;
                showError(inst.handleError(err));
            }
        }

        // ---- start flow ----
        function setStarting(on) { inst.trainingStarting = !!on; inst.recompute(); }

        function showRun() { if (runArea && typeof runArea.focus === 'function') runArea.focus(); }

        async function startTraining() {
            if (disposed || inst.trainingStarting || inst.readonly || !inst.project || !inst.preflight || !inst.preflight.ready) return;
            if (!P.need(inst, 'table')) return;
            setStarting(true);
            showError('');
            const id = inst.project.id, preset = currentPreset();
            let prep;
            try {
                prep = await inst.api.prepareRun(id, preset);
            } catch (err) {
                if (!disposed) setStarting(false);
                if (disposed || (err && err.aborted)) return;
                startFailed(err);
                return;
            }
            if (disposed || !inst.project || inst.project.id !== id) { setStarting(false); return; }
            openConfirm(id, preset, prep);
        }

        // A refusal that means the list is out of date reloads it (and the runs), and says why above the list.
        function startFailed(err) {
            showError(inst.handleError(err));
            const code = P.errors.normalize(err).code;
            if (code === 'preflight_failed' || code === 'license_consent_required' || code === 'run_active') loadPreflight();
            if (code === 'run_active') inst.reloadRuns();
        }

        function openConfirm(id, preset, first) {
            const dlg = P.table.makeDialog(inst, t('chibi_sensei.confirm.title'), 'confirm-dialog', true);
            const confirmBody = el('div', { dataset: { csn: 'confirm-body' } });
            const notice = el('p', { class: 'csn-note', role: 'status', dataset: { csn: 'confirm-expired' } });
            const err = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'confirm-error' } });
            notice.hidden = true; err.hidden = true;
            const cancel = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'confirm-cancel' }, text: t('chibi_sensei.confirm.cancel'), onClick: () => dlg.close() });
            const start = el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'confirm-start' }, text: t('chibi_sensei.confirm.start') });
            dlg.content.appendChild(notice);
            dlg.content.appendChild(confirmBody);
            dlg.content.appendChild(err);
            dlg.content.appendChild(el('div', { class: 'csn-actions csn-dialog-actions' }, cancel, start));
            let prep = first, busy = false, done = false;
            const fill = p => { P.clear(confirmBody); confirmBody.appendChild(confirmDetails(inst, p)); };
            fill(prep);
            dlg.onClose = () => { if (!done && !disposed) setStarting(false); };
            start.addEventListener('click', async () => {
                if (busy) return;
                busy = true; start.disabled = true; cancel.disabled = true; start.textContent = t('chibi_sensei.pending'); err.hidden = true;
                let refocus = false;
                try {
                    const run = await inst.api.startRun(id, prep.plan_token);
                    if (disposed) return;
                    done = true;
                    dlg.close();
                    setStarting(false);
                    inst.applyRun(run);
                    inst.notify(t('chibi_sensei.confirm.started'), 'info');
                    showRun();
                } catch (e) {
                    if (disposed || (e && e.aborted)) return;
                    const code = P.errors.normalize(e).code;
                    if (code === 'plan_expired') {
                        // The token was unknown, expired or the plan changed: prepare again here and say so; the user confirms again.
                        try {
                            prep = await inst.api.prepareRun(id, preset);
                            if (disposed) return;
                            fill(prep);
                            notice.textContent = t('chibi_sensei.confirm.expired');
                            notice.hidden = false;
                            refocus = true;
                        } catch (e2) {
                            if (disposed || (e2 && e2.aborted)) return;
                            dlg.close();
                            startFailed(e2);
                        }
                    } else if (code === 'preflight_failed' || code === 'run_active' || code === 'license_consent_required') {
                        dlg.close();
                        startFailed(e);
                    } else {
                        err.textContent = inst.handleError(e); err.hidden = false;
                    }
                } finally {
                    busy = false;
                    if (!done && !disposed) {
                        start.disabled = false; cancel.disabled = false; start.textContent = t('chibi_sensei.confirm.start');
                        if (refocus) start.focus();
                    }
                }
            });
            dlg.open();
            start.focus();
        }

        // ---- wiring ----
        const again = P.debounce(refresh, REFRESH_MS);
        const againPreflight = P.debounce(loadPreflight, PREFLIGHT_REFRESH_MS);
        const offs = [
            inst.bus.on('rows', again),
            inst.bus.on('checks', again),
            inst.bus.on('project', () => {
                if (inst.project && inst.project.data_rev !== loadedRev) again(); else paintAll();
            }),
            inst.bus.on('readonly', () => { paintAll(); pf.repaint(); }),
            inst.bus.on('run', run => { arrange(); if (run && P.runTerminal(run.state)) againPreflight(); }),
            inst.bus.on('runs', arrange)
        ];
        arrange();
        refresh();
        return {
            el: node, refresh, startTraining, showRun,
            dispose() {
                disposed = true;
                again.cancel();
                againPreflight.cancel();
                offs.forEach(off => { if (typeof off === 'function') off(); });
                if (runArea && typeof runArea.dispose === 'function') runArea.dispose();
                inst.preflight = null;
                inst.trainingStarting = false;
            }
        };
    }

    // Test and Use: a short state card; the mascot rests.
    function createLater(inst, step) {
        const { el } = P;
        const t = inst.t;
        const art = P.mascot ? P.mascot.create({ size: 88, mood: 'idle' }).el : null;
        const node = el('section', { class: 'csn-later', role: 'status', dataset: { csn: 'later', step: step } },
            art, el('h2', { text: t('chibi_sensei.later.title') }), el('p', { text: t(LATER_KEYS[step] || LATER_KEYS.test) }));
        return { el: node, dispose() {} };
    }

    P.training = { create, createLater, groupPreflight, enableFlags, validateOverrides, seqOptions, PRESETS };
})();
