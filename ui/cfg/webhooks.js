// cfg/webhooks.js — Webhooks, Tokens, Log & Outgoing section module

let whWebhooks = [];
let whTokens = [];
let whPresets = [];
let whLog = [];
let whEditingId = null; // null = new, string = editing
let ogWebhooks = [];    // outgoing webhooks
let ogEditingIdx = -1;  // -1 = new, >= 0 = editing index

async function whFetchAll() {
    try {
        const [wResp, tResp, pResp, ogResp] = await Promise.all([
            fetch('/api/webhooks'), fetch('/api/tokens'), fetch('/api/webhooks/presets'), fetch('/api/outgoing-webhooks')
        ]);
        if (wResp.ok) whWebhooks = await wResp.json();
        if (tResp.ok) whTokens = await tResp.json();
        if (pResp.ok) whPresets = await pResp.json();
        if (ogResp.ok) ogWebhooks = await ogResp.json();
    } catch (e) { console.error('webhook fetch error', e); }
}

async function renderWebhooksSection(section) {
    const content = document.getElementById('content');
    content.innerHTML = '<div class="cfg-section active"><div class="wh-loading-state">' + t('config.webhooks.loading') + '</div></div>';
    await whFetchAll();

    // Check if webhooks are enabled in config
    const enabled = configData.webhooks && configData.webhooks.enabled;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    if (!enabled) {
        html += `<div class="wh-notice">
                    <span>⚠️</span>
                    <div>
                        <strong>${t('config.webhooks.disabled_notice')}</strong><br>
                        <small>${t('config.webhooks.disabled_desc')}</small>
                    </div>
                </div>`;
    }

    // Config settings (enabled, max_payload_size, rate_limit)
    html += whRenderConfigSettings();

    if (enabled) {
        // Tabs: Webhooks | Tokens | Log
        html += `<div class="wh-tabs">
                    <button class="wh-tab active" onclick="whSwitchTab(this,'wh-panel-hooks')">${t('config.webhooks.tab_webhooks')}</button>
                    <button class="wh-tab" onclick="whSwitchTab(this,'wh-panel-tokens')">${t('config.webhooks.tab_tokens')}</button>
                    <button class="wh-tab" onclick="whSwitchTab(this,'wh-panel-log')">${t('config.webhooks.tab_log')}</button>
                </div>`;

        // Webhooks panel
        html += '<div id="wh-panel-hooks" class="wh-panel active">';
        html += whRenderWebhookList();
        html += '</div>';

        // Tokens panel
        html += '<div id="wh-panel-tokens" class="wh-panel">';
        html += whRenderTokenList();
        html += '</div>';

        // Log panel
        html += '<div id="wh-panel-log" class="wh-panel">';
        html += whRenderLog();
        html += '</div>';
    }

    // Outgoing webhooks — always visible, independent of incoming webhook server
    html += `<div class="wh-outgoing-wrap">
        <div class="wh-section-title">
            ${t('config.webhooks.tab_outgoing')}
        </div>`;
    html += ogRenderList();
    html += '</div>';

    html += '</div>';
    content.innerHTML = html;

    // Load log if enabled
    if (enabled) whLoadLog();
}

function whRenderConfigSettings() {
    const wh = configData.webhooks || {};
    return `
            <div class="wh-settings-wrap">
                <div class="wh-section-title">
                    ${t('config.webhooks.settings_title')}
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.webhooks.config_enabled_label')}</div>
                    <div class="toggle-wrap">
                        <div class="toggle ${wh.enabled ? 'on' : ''}" data-path="webhooks.enabled" onclick="toggleBool(this);markDirty()"></div>
                        <span class="toggle-label">${wh.enabled ? t('config.webhooks.toggle_active') : t('config.webhooks.toggle_inactive')}</span>
                    </div>
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.webhooks.max_payload_label')}</div>
                    <input class="field-input" type="number" step="1" data-path="webhooks.max_payload_size" value="${wh.max_payload_size || 65536}" onchange="markDirty()">
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.webhooks.rate_limit_label')}</div>
                    <input class="field-input" type="number" step="1" data-path="webhooks.rate_limit" value="${wh.rate_limit || 0}" onchange="markDirty()">
                </div>
            </div>`;
}

function whSwitchTab(btn, panelId) {
    document.querySelectorAll('.wh-tab').forEach(tab => tab.classList.remove('active'));
    document.querySelectorAll('.wh-panel').forEach(p => p.classList.remove('active'));
    btn.classList.add('active');
    const panel = document.getElementById(panelId);
    if (panel) panel.classList.add('active');
    if (panelId === 'wh-panel-log') whLoadLog();
}

/* ── Webhook List ── */
function whRenderWebhookList() {
    let html = `<div class="wh-toolbar">
                <span class="wh-count">${whWebhooks.length} / ${10} ${t('config.webhooks.count_webhooks')}</span>
                ${whWebhooks.length < 10 ? `<button class="wh-btn wh-btn-primary" onclick="whShowEditor(null)">+ ${t('config.webhooks.new_webhook')}</button>` : ''}
            </div>`;

    if (whWebhooks.length === 0) {
        html += `<div class="wh-empty">${t('config.webhooks.empty')}</div>`;
    } else {
        html += '<div class="wh-list">';
        for (const w of whWebhooks) {
            const url = location.origin + '/webhook/' + w.slug;
            html += `<div class="wh-card ${w.enabled ? '' : 'wh-card-disabled'}">
                        <div class="wh-card-header">
                            <div class="wh-card-title">
                                <span class="wh-status-dot ${w.enabled ? 'active' : ''}"></span>
                                <strong>${esc(w.name)}</strong>
                                <span class="wh-card-slug">/${w.slug}</span>
                            </div>
                            <div class="wh-card-actions">
                                <button class="wh-btn-icon" title="${t('config.webhooks.action_test')}" onclick="whTestWebhook('${escapeAttr(w.id)}')">🧪</button>
                                <button class="wh-btn-icon" title="${t('config.webhooks.action_edit')}" onclick="whShowEditor('${escapeAttr(w.id)}')">✏️</button>
                                <button class="wh-btn-icon wh-btn-danger" title="${t('config.webhooks.action_delete')}" onclick="whDeleteWebhook('${escapeAttr(w.id)}','${escapeAttr(w.name)}')">🗑️</button>
                            </div>
                        </div>
                        <div class="wh-card-body">
                            <div class="wh-card-url" onclick="whCopy(this, '${escapeAttr(url)}')" title="${t('config.webhooks.click_to_copy')}">
                                <code>${esc(url)}</code>
                                <span class="wh-copy-icon">📋</span>
                            </div>
                            <div class="wh-card-meta">
                                <span>${t('config.webhooks.card_mode')} <strong>${w.delivery?.mode || 'message'}</strong></span>
                                <span>${t('config.webhooks.card_fires')} <strong>${w.fire_count || 0}</strong></span>
                                ${w.format?.description ? '<span>' + esc(w.format.description) + '</span>' : ''}
                            </div>
                        </div>
                    </div>`;
        }
        html += '</div>';
    }

    // Hidden editor form
    html += '<div id="wh-editor" class="wh-editor is-hidden"></div>';
    return html;
}

/* ── Webhook Editor ── */
function whShowEditor(id) {
    whEditingId = id;
    const wh = id ? whWebhooks.find(w => w.id === id) : null;
    const ed = document.getElementById('wh-editor');
    if (!ed) return;

    const name = wh ? wh.name : '';
    const slug = wh ? wh.slug : '';
    const enabled = wh ? wh.enabled : true;
    const tokenId = wh ? (wh.token_id || '') : '';
    const format = wh ? wh.format : { accepted_content_types: ['application/json'], fields: [], description: '' };
    const delivery = wh ? wh.delivery : { mode: 'message', prompt_template: '', priority: 'queue' };

    let presetsHtml = '<option value="">— ' + t('config.webhooks.choose_preset') + ' —</option>';
    for (const p of whPresets) {
        presetsHtml += `<option value="${p.key}">${esc(p.label)}</option>`;
    }

    let tokensHtml = '<option value="">— ' + t('config.webhooks.no_token') + ' —</option>';
    for (const tok of whTokens) {
        tokensHtml += `<option value="${tok.id}" ${tok.id === tokenId ? 'selected' : ''}>${esc(tok.name)} (${tok.prefix}…)</option>`;
    }

    let fieldsHtml = '';
    if (format.fields && format.fields.length > 0) {
        for (let i = 0; i < format.fields.length; i++) {
            fieldsHtml += whFieldRow(i, format.fields[i]);
        }
    }

    ed.innerHTML = `
            <div class="wh-editor-header">
                <h3>${id ? t('config.webhooks.edit_webhook') : t('config.webhooks.new_webhook')}</h3>
                <button class="wh-btn-icon" onclick="whHideEditor()">✕</button>
            </div>
            <div class="wh-editor-body">
                <div class="wh-form-row">
                    <label>${t('config.webhooks.name_label')}</label>
                    <input id="wh-f-name" class="field-input" value="${esc(name)}" placeholder="${t('config.webhooks.name_placeholder')}">
                </div>
                <div class="wh-form-row">
                    <label>${t('config.webhooks.slug_label')}</label>
                    <input id="wh-f-slug" class="field-input" value="${esc(slug)}" placeholder="${t('config.webhooks.slug_placeholder')}" pattern="[a-z0-9][a-z0-9-]{1,48}[a-z0-9]" ${id ? 'disabled' : ''}>
                    <small class="wh-field-hint">${t('config.webhooks.slug_hint')}</small>
                </div>
                <div class="wh-form-row">
                    <label>${t('config.webhooks.enabled_label')}</label>
                    <div class="toggle-wrap">
                        <div id="wh-f-enabled" class="toggle ${enabled ? 'on' : ''}" onclick="this.classList.toggle('on')"></div>
                    </div>
                </div>
                <div class="wh-form-row">
                    <label>${t('config.webhooks.token_label')}</label>
                    <select id="wh-f-token" class="field-select">${tokensHtml}</select>
                </div>
                <div class="wh-form-row">
                    <label>${t('config.webhooks.preset_label')}</label>
                    <select id="wh-f-preset" class="field-select" onchange="whApplyPreset(this.value)">${presetsHtml}</select>
                </div>
                <div class="wh-subsection-title">${t('config.webhooks.format_section')}</div>
                <div class="wh-form-row">
                    <label>${t('config.webhooks.content_types_label')}</label>
                    <input id="wh-f-ct" class="field-input" value="${(format.accepted_content_types || []).join(', ')}" placeholder="${t('config.webhooks.ct_placeholder')}">
                </div>
                <div class="wh-form-row">
                    <label>${t('config.webhooks.description_label')}</label>
                    <input id="wh-f-desc" class="field-input" value="${esc(format.description || '')}">
                </div>
                <div class="wh-form-row">
                    <label>${t('config.webhooks.signature_header_label')}</label>
                    <input id="wh-f-sighdr" class="field-input" value="${esc(format.signature_header || '')}" placeholder="${t('config.webhooks.sighdr_placeholder')}">
                </div>
                <div class="wh-form-row">
                    <label>${t('config.webhooks.signature_algo_label')}</label>
                    <select id="wh-f-sigalgo" class="field-select">
                        <option value="" ${!format.signature_algo ? 'selected' : ''}>— ${t('config.webhooks.sig_none')} —</option>
                        <option value="sha256" ${format.signature_algo === 'sha256' ? 'selected' : ''}>${t('config.webhooks.sig_sha256')}</option>
                        <option value="sha1" ${format.signature_algo === 'sha1' ? 'selected' : ''}>${t('config.webhooks.sig_sha1')}</option>
                        <option value="plain" ${format.signature_algo === 'plain' ? 'selected' : ''}>${t('config.webhooks.sig_plain')}</option>
                    </select>
                </div>
                <div class="wh-form-row">
                    <label>${t('config.webhooks.signature_secret_label')}</label>
                    <input id="wh-f-sigsec" class="field-input" type="password" value="${esc(format.signature_secret || '')}" placeholder="${t('config.webhooks.sig_secret_placeholder')}">
                </div>
                <div class="wh-subsection-title">
                    ${t('config.webhooks.field_mappings_label')}
                    <button class="wh-btn wh-btn-sm wh-field-add-btn" onclick="whAddFieldRow()">${t('config.webhooks.add_field_button')}</button>
                </div>
                <div id="wh-fields-list">${fieldsHtml}</div>
                <div class="wh-subsection-title">${t('config.webhooks.delivery_section')}</div>
                <div class="wh-form-row">
                    <label>${t('config.webhooks.mode_label')}</label>
                    <select id="wh-f-mode" class="field-select">
                        <option value="message" ${delivery.mode === 'message' ? 'selected' : ''}>${t('config.webhooks.mode_message')}</option>
                        <option value="notify" ${delivery.mode === 'notify' ? 'selected' : ''}>${t('config.webhooks.mode_notify')}</option>
                        <option value="silent" ${delivery.mode === 'silent' ? 'selected' : ''}>${t('config.webhooks.mode_silent')}</option>
                    </select>
                </div>
                <div class="wh-form-row">
                    <label>${t('config.webhooks.prompt_template_label')}</label>
                    <textarea id="wh-f-prompt" class="field-input wh-textarea" rows="5" placeholder="${t('config.webhooks.prompt_placeholder')}">${esc(delivery.prompt_template || '')}</textarea>
                </div>
                <div class="wh-form-row">
                    <label>${t('config.webhooks.priority_label')}</label>
                    <select id="wh-f-priority" class="field-select">
                        <option value="queue" ${delivery.priority !== 'immediate' ? 'selected' : ''}>${t('config.webhooks.priority_queue')}</option>
                        <option value="immediate" ${delivery.priority === 'immediate' ? 'selected' : ''}>${t('config.webhooks.priority_immediate')}</option>
                    </select>
                </div>
            </div>
            <div class="wh-editor-footer">
                <button class="wh-btn" onclick="whHideEditor()">${t('config.webhooks.cancel')}</button>
                <button class="wh-btn wh-btn-primary" onclick="whSaveWebhook()">${t('config.webhooks.save')}</button>
            </div>`;
    setHidden(ed, false);
    ed.scrollIntoView({ behavior: 'smooth', block: 'start' });
}

function whHideEditor() {
    const ed = document.getElementById('wh-editor');
    setHidden(ed, true);
    whEditingId = null;
}

function whFieldRow(idx, field) {
    return `<div class="wh-field-row" data-idx="${idx}">
                <input class="field-input wh-field-src" value="${esc(field?.source || '')}" placeholder="${t('config.webhooks.field_source_placeholder')}">
                <input class="field-input wh-field-alias" value="${esc(field?.alias || '')}" placeholder="${t('config.webhooks.field_alias_placeholder')}">
                <button class="wh-btn-icon wh-btn-danger" onclick="this.parentElement.remove()">✕</button>
            </div>`;
}

function whAddFieldRow() {
    const list = document.getElementById('wh-fields-list');
    if (!list) return;
    const idx = list.children.length;
    list.insertAdjacentHTML('beforeend', whFieldRow(idx, {}));
}

function whApplyPreset(key) {
    const preset = whPresets.find(p => p.key === key);
    if (!preset) return;
    const f = preset.format;
    document.getElementById('wh-f-ct').value = (f.accepted_content_types || []).join(', ');
    document.getElementById('wh-f-desc').value = f.description || '';
    document.getElementById('wh-f-sighdr').value = f.signature_header || '';
    document.getElementById('wh-f-sigalgo').value = f.signature_algo || '';
    // Populate fields
    const list = document.getElementById('wh-fields-list');
    list.innerHTML = '';
    if (f.fields) {
        for (let i = 0; i < f.fields.length; i++) {
            list.insertAdjacentHTML('beforeend', whFieldRow(i, f.fields[i]));
        }
    }
    // Prompt hint
    if (preset.prompt_hint) {
        document.getElementById('wh-f-prompt').value = preset.prompt_hint;
    }
}

function whCollectEditor() {
    const fields = [];
    document.querySelectorAll('#wh-fields-list .wh-field-row').forEach(row => {
        const src = row.querySelector('.wh-field-src')?.value?.trim();
        const alias = row.querySelector('.wh-field-alias')?.value?.trim();
        if (src) fields.push({ source: src, alias: alias || '' });
    });
    const ct = document.getElementById('wh-f-ct').value.split(',').map(s => s.trim()).filter(Boolean);
    return {
        name: document.getElementById('wh-f-name').value.trim(),
        slug: document.getElementById('wh-f-slug').value.trim(),
        enabled: document.getElementById('wh-f-enabled').classList.contains('on'),
        token_id: document.getElementById('wh-f-token').value,
        format: {
            accepted_content_types: ct.length ? ct : ['application/json'],
            fields: fields,
            description: document.getElementById('wh-f-desc').value.trim(),
            signature_header: document.getElementById('wh-f-sighdr').value.trim(),
            signature_algo: document.getElementById('wh-f-sigalgo').value,
            signature_secret: document.getElementById('wh-f-sigsec').value,
        },
        delivery: {
            mode: document.getElementById('wh-f-mode').value,
            prompt_template: document.getElementById('wh-f-prompt').value,
            priority: document.getElementById('wh-f-priority').value,
        }
    };
}

async function whSaveWebhook() {
    const data = whCollectEditor();
    if (!data.name) return showToast(t('config.webhooks.name_required'), 'error');
    if (!whEditingId && !data.slug) return showToast(t('config.webhooks.slug_required'), 'error');

    try {
        let resp;
        if (whEditingId) {
            resp = await fetch('/api/webhooks/' + whEditingId, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });
        } else {
            resp = await fetch('/api/webhooks', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });
        }
        const result = await resp.json();
        if (!resp.ok) {
            showToast(result.error || t('config.webhooks.error'), 'error');
            return;
        }
        showToast(t('config.webhooks.saved'), 'success');
        whHideEditor();
        // Refresh
        const section = SECTIONS.flatMap(g => g.items).find(s => s.key === 'webhooks');
        await renderWebhooksSection(section);
    } catch (e) {
        showToast(e.message, 'error');
    }
}

async function whDeleteWebhook(id, name) {
    if (!(await showConfirm(t('config.webhooks.delete_confirm_title'), t('config.webhooks.delete_confirm') + name + '?'))) return;
    try {
        const resp = await fetch('/api/webhooks/' + id, { method: 'DELETE' });
        if (resp.ok) {
            showToast(t('config.webhooks.deleted'), 'success');
            const section = SECTIONS.flatMap(g => g.items).find(s => s.key === 'webhooks');
            await renderWebhooksSection(section);
        } else {
            const r = await resp.json();
            showToast(r.error || t('config.webhooks.error'), 'error');
        }
    } catch (e) { showToast(e.message, 'error'); }
}

async function whTestWebhook(id) {
    try {
        const resp = await fetch('/api/webhooks/' + id + '/test', { method: 'POST' });
        const r = await resp.json();
        if (resp.ok) {
            showToast(t('config.webhooks.test_prefix') + (r.prompt || t('config.webhooks.test_ok')).substring(0, 100), 'success');
        } else {
            showToast(r.error || t('config.webhooks.test_failed'), 'error');
        }
    } catch (e) { showToast(e.message, 'error'); }
}

/* ── Token Management ── */
function whRenderTokenList() {
    let html = `<div class="wh-toolbar">
                <span class="wh-count">${whTokens.length} ${t('config.webhooks.count_tokens')}</span>
                <button class="wh-btn wh-btn-primary" onclick="whCreateToken()">+ ${t('config.tokens.new_token')}</button>
            </div>`;
    html += '<div id="wh-token-created" class="wh-token-reveal is-hidden"></div>';

    if (whTokens.length === 0) {
        html += `<div class="wh-empty">${t('config.tokens.empty')}</div>`;
    } else {
        html += '<div class="wh-list">';
        for (const tok of whTokens) {
            const exp = tok.expires_at ? new Date(tok.expires_at).toLocaleDateString() : t('config.tokens.never');
            const lastUsed = tok.last_used_at ? new Date(tok.last_used_at).toLocaleString() : '—';
            html += `<div class="wh-card wh-card-token ${tok.enabled ? '' : 'wh-card-disabled'}">
                        <div class="wh-card-header">
                            <div class="wh-card-title">
                                <span class="wh-status-dot ${tok.enabled ? 'active' : ''}"></span>
                                <strong>${esc(tok.name)}</strong>
                                <code class="wh-token-prefix">${esc(tok.prefix)}…</code>
                            </div>
                            <div class="wh-card-actions">
                                <button class="wh-btn-icon" title="${tok.enabled ? t('config.tokens.toggle_disable') : t('config.tokens.toggle_enable')}" onclick="whToggleToken('${tok.id}', ${!tok.enabled})">${tok.enabled ? '🔒' : '🔓'}</button>
                                <button class="wh-btn-icon wh-btn-danger" title="${t('config.webhooks.action_delete')}" onclick="whDeleteToken('${tok.id}','${esc(tok.name)}')">🗑️</button>
                            </div>
                        </div>
                        <div class="wh-card-meta wh-card-meta-spaced">
                            <span>${t('config.tokens.scopes_label')} <strong>${(tok.scopes || ['webhook']).join(', ')}</strong></span>
                            <span>${t('config.tokens.expires')}: <strong>${exp}</strong></span>
                            <span>${t('config.tokens.last_used')}: ${lastUsed}</span>
                        </div>
                    </div>`;
        }
        html += '</div>';
    }
    return html;
}

async function whCreateToken() {
    const name = prompt(t('config.tokens.name_prompt'));
    if (!name) return;
    try {
        const resp = await fetch('/api/tokens', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name: name, scopes: ['webhook'] })
        });
        const r = await resp.json();
        if (!resp.ok) { showToast(r.error || 'Error', 'error'); return; }
        // Show the raw token ONCE
        const reveal = document.getElementById('wh-token-created');
        if (reveal) {
            reveal.style.display = 'block';
            reveal.innerHTML = `
                        <div class="wh-token-alert">
                            <strong>⚠️ ${t('config.tokens.created_notice')}</strong>
                            <div class="wh-token-value" onclick="whCopy(this, '${esc(r.raw_token || r.token)}')" title="${t('config.webhooks.click_to_copy')}">
                                <code>${esc(r.raw_token || r.token)}</code>
                                <span class="wh-copy-icon">📋</span>
                            </div>
                        </div>`;
            reveal.scrollIntoView({ behavior: 'smooth' });
        }
        // Refresh tokens
        const tResp = await fetch('/api/tokens');
        if (tResp.ok) whTokens = await tResp.json();
        // Re-render token list below the reveal
        const panel = document.getElementById('wh-panel-tokens');
        if (panel) {
            const revealHtml = reveal.outerHTML;
            panel.innerHTML = whRenderTokenList();
            // Re-insert reveal
            const newReveal = document.getElementById('wh-token-created');
            if (newReveal) {
                newReveal.outerHTML = revealHtml;
            }
        }
    } catch (e) { showToast(e.message, 'error'); }
}

async function whToggleToken(id, enabled) {
    try {
        const resp = await fetch('/api/tokens/' + id, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ enabled: enabled })
        });
        if (resp.ok) {
            showToast(enabled ? t('config.tokens.activated') : t('config.tokens.deactivated'), 'success');
            const tResp = await fetch('/api/tokens');
            if (tResp.ok) whTokens = await tResp.json();
            const panel = document.getElementById('wh-panel-tokens');
            if (panel) panel.innerHTML = whRenderTokenList();
        }
    } catch (e) { showToast(e.message, 'error'); }
}

async function whDeleteToken(id, name) {
    if (!(await showConfirm(t('config.tokens.delete_confirm_title'), t('config.tokens.delete_confirm') + name + '?'))) return;
    try {
        const resp = await fetch('/api/tokens/' + id, { method: 'DELETE' });
        if (resp.ok) {
            showToast(t('config.tokens.deleted'), 'success');
            const tResp = await fetch('/api/tokens');
            if (tResp.ok) whTokens = await tResp.json();
            const panel = document.getElementById('wh-panel-tokens');
            if (panel) panel.innerHTML = whRenderTokenList();
        }
    } catch (e) { showToast(e.message, 'error'); }
}

/* ── Webhook Log ── */
function whRenderLog() {
    return `<div id="wh-log-content" class="wh-log-content">
                <div class="wh-log-loading">${t('config.webhooks.log_loading')}</div>
            </div>`;
}

async function whLoadLog() {
    try {
        const resp = await fetch('/api/webhooks/log');
        if (!resp.ok) return;
        whLog = await resp.json();
        const el = document.getElementById('wh-log-content');
        if (!el) return;
        if (!whLog || whLog.length === 0) {
            el.innerHTML = `<div class="wh-empty">${t('config.webhooks.log_empty')}</div>`;
            return;
        }
        let html = '<table class="wh-log-table"><thead><tr><th>' + t('config.webhooks.log_col_time') + '</th><th>' + t('config.webhooks.log_col_webhook') + '</th><th>' + t('config.webhooks.log_col_status') + '</th><th>' + t('config.webhooks.log_col_ip') + '</th><th>' + t('config.webhooks.log_col_size') + '</th><th>' + t('config.webhooks.log_col_delivered') + '</th></tr></thead><tbody>';
        for (const e of whLog) {
            const time = new Date(e.timestamp).toLocaleString();
            const statusClass = e.status_code >= 200 && e.status_code < 300 ? 'success' : 'error';
            html += `<tr>
                        <td>${time}</td>
                        <td>${esc(e.webhook_name || e.webhook_id)}</td>
                        <td><span class="wh-badge ${statusClass}">${e.status_code}</span></td>
                        <td>${esc(e.source_ip || '')}</td>
                        <td>${e.payload_size || 0} ${t('config.webhooks.payload_bytes')}</td>
                        <td>${e.delivered ? t('config.webhooks.log_delivered') : (e.error ? t('config.webhooks.log_failed') + ' ' + esc(e.error) : t('config.webhooks.log_pending'))}</td>
                    </tr>`;
        }
        html += '</tbody></table>';
        el.innerHTML = html;
    } catch (e) {
        const el = document.getElementById('wh-log-content');
        if (el) el.innerHTML = '<div class="wh-empty wh-empty-danger">' + t('config.webhooks.log_error_prefix') + esc(e.message) + '</div>';
    }
}

/* ── Utilities ── */
// esc(s) is defined globally in config.html — not duplicated here.

function whCopy(el, text) {
    navigator.clipboard.writeText(text).then(() => {
        const icon = el.querySelector('.wh-copy-icon');
        if (icon) { icon.textContent = '✓'; setTimeout(() => icon.textContent = '📋', 2000); }
    });
}

/* ══════════════════════════════════════
   Outgoing Webhooks — CRUD UI
   ══════════════════════════════════════ */

function ogRenderList() {
    let html = `<div class="wh-toolbar">
        <span class="wh-count">${ogWebhooks.length} ${t('config.webhooks.tab_outgoing')}</span>
        <button class="wh-btn wh-btn-primary" onclick="ogShowModal(-1)">+ ${t('config.webhooks.og_new')}</button>
    </div>`;
    if (!ogWebhooks || ogWebhooks.length === 0) {
        html += `<div class="wh-empty">${t('config.webhooks.og_empty')}</div>`;
    } else {
        html += '<div class="wh-list">';
        for (let i = 0; i < ogWebhooks.length; i++) {
            const w = ogWebhooks[i];
            const methodBadge = `<span class="wh-badge ${(w.method || 'POST').toLowerCase()}">${w.method || 'POST'}</span>`;
            const paramCount = (w.parameters || []).length;
            html += `<div class="wh-card">
                <div class="wh-card-header">
                    <div class="wh-card-title">${methodBadge} <strong>${esc(w.name || t('config.webhooks.og_unnamed'))}</strong></div>
                    <div class="wh-card-actions">
                        <button class="wh-btn-icon" title="${t('config.webhooks.action_edit')}" onclick="ogShowModal(${i})">\u270f\ufe0f</button>
                        <button class="wh-btn-icon wh-btn-danger" title="${t('config.webhooks.action_delete')}" onclick="ogDelete(${i})">\ud83d\uddd1\ufe0f</button>
                    </div>
                </div>
                <div class="wh-card-body">
                    <div class="wh-card-url wh-card-url-static"><code>${esc(w.url || '')}</code></div>
                    <div class="wh-card-meta">
                        <span>${esc(w.description || '')}</span>
                        <span>${paramCount} ${t('config.webhooks.og_params')}</span>
                        <span>${t('config.webhooks.og_payload_type')}: <strong>${w.payload_type || 'json'}</strong></span>
                    </div>
                </div>
            </div>`;
        }
        html += '</div>';
    }
    html += '<div id="og-modal-overlay" class="og-modal-overlay"></div>';
    return html;
}

function ogShowModal(idx) {
    ogEditingIdx = idx;
    const w = idx >= 0 ? ogWebhooks[idx] : {};
    const overlay = document.getElementById('og-modal-overlay');
    if (!overlay) return;
    const name = w.name || '', desc = w.description || '', method = w.method || 'POST';
    const url = w.url || '', payloadType = w.payload_type || 'json', bodyTemplate = w.body_template || '';
    const headers = w.headers || {}, params = w.parameters || [];
    let headersStr = Object.entries(headers).map(([k, v]) => k + ': ' + v).join('\n');
    let paramsHtml = '';
    for (let i = 0; i < params.length; i++) paramsHtml += ogParamRow(i, params[i]);

    overlay.innerHTML = `
    <div class="og-modal">
        <div class="og-modal-header">
            <h3>${idx >= 0 ? t('config.webhooks.og_edit') : t('config.webhooks.og_new')}</h3>
            <button class="wh-btn-icon" onclick="ogCloseModal()">\u2715</button>
        </div>
        <div class="og-modal-body">
            <div class="wh-form-row">
                <label>${t('config.webhooks.og_name')}</label>
                <input id="og-f-name" class="field-input" value="${esc(name)}" placeholder="${t('config.webhooks.og_name_placeholder')}">
                <small class="wh-field-hint">${t('config.webhooks.og_name_hint')}</small>
            </div>
            <div class="wh-form-row">
                <label>${t('config.webhooks.og_desc')}</label>
                <input id="og-f-desc" class="field-input" value="${esc(desc)}" placeholder="${t('config.webhooks.og_desc_placeholder')}">
                <small class="wh-field-hint">${t('config.webhooks.og_desc_hint')}</small>
            </div>
            <div class="wh-form-row og-form-split">
                <div><label>${t('config.webhooks.og_method')}</label>
                    <select id="og-f-method" class="field-select">
                        <option value="GET" ${method === 'GET' ? 'selected' : ''}>${t('config.webhooks.method_get')}</option>
                        <option value="POST" ${method === 'POST' ? 'selected' : ''}>${t('config.webhooks.method_post')}</option>
                        <option value="PUT" ${method === 'PUT' ? 'selected' : ''}>${t('config.webhooks.method_put')}</option>
                        <option value="DELETE" ${method === 'DELETE' ? 'selected' : ''}>${t('config.webhooks.method_delete')}</option>
                    </select>
                </div>
                <div class="og-col-url"><label>${t('config.webhooks.og_url')}</label>
                    <input id="og-f-url" class="field-input" value="${esc(url)}" placeholder="${t('config.webhooks.og_url_placeholder')}">
                </div>
            </div>
            <div class="wh-form-row">
                <label>${t('config.webhooks.og_headers')} <small>${t('config.webhooks.og_headers_hint')}</small></label>
                <textarea id="og-f-headers" class="field-input wh-textarea" rows="3" placeholder="${t('config.webhooks.og_headers_placeholder')}">${esc(headersStr)}</textarea>
            </div>
            <div class="wh-subsection-title wh-subsection-title-inline">
                ${t('config.webhooks.og_parameters')}
                <button class="wh-btn wh-btn-sm" onclick="ogAddParam()">+ ${t('config.webhooks.og_add_param')}</button>
            </div>
            <div id="og-params-list">${paramsHtml}</div>
            <div class="wh-form-row og-form-split og-form-row-spaced">
                <div><label>${t('config.webhooks.og_payload_type')}</label>
                    <select id="og-f-ptype" class="field-select" onchange="ogToggleTemplate()">
                        <option value="json" ${payloadType === 'json' ? 'selected' : ''}>${t('config.webhooks.og_auto_json')}</option>
                        <option value="custom" ${payloadType === 'custom' ? 'selected' : ''}>${t('config.webhooks.og_custom_template')}</option>
                    </select>
                </div>
                <div class="og-col-body"><label>${t('config.webhooks.og_body_template')}</label>
                    <textarea id="og-f-body" class="field-input wh-textarea" rows="3" placeholder="${t('config.webhooks.og_body_template_placeholder')}" ${payloadType !== 'custom' ? 'disabled' : ''}>${esc(bodyTemplate)}</textarea>
                </div>
            </div>
        </div>
        <div class="og-modal-footer">
            <button class="wh-btn" onclick="ogCloseModal()">${t('config.webhooks.cancel')}</button>
            <button class="wh-btn wh-btn-primary" onclick="ogSave()">${t('config.webhooks.save')}</button>
        </div>
    </div>`;
    overlay.style.display = 'flex';
}

function ogCloseModal() {
    const o = document.getElementById('og-modal-overlay');
    if (o) o.style.display = 'none';
    ogEditingIdx = -1;
}

function ogParamRow(idx, p) {
    return `<div class="og-param-row" data-idx="${idx}">
        <input class="field-input og-p-name" value="${esc(p.name || '')}" placeholder="${t('config.webhooks.og_param_name_placeholder')}">
        <select class="field-select og-p-type">
            <option value="string" ${(p.type || 'string') === 'string' ? 'selected' : ''}>${t('config.webhooks.param_type_string')}</option>
            <option value="number" ${p.type === 'number' ? 'selected' : ''}>${t('config.webhooks.param_type_number')}</option>
            <option value="boolean" ${p.type === 'boolean' ? 'selected' : ''}>${t('config.webhooks.param_type_boolean')}</option>
        </select>
        <input class="field-input og-p-desc og-p-desc-wide" value="${esc(p.description || '')}" placeholder="${t('config.webhooks.og_param_desc_placeholder')}">
        <div class="toggle-wrap og-toggle-wrap">
            <div class="toggle ${p.required ? 'on' : ''}" onclick="this.classList.toggle('on')" title="${t('config.webhooks.og_required_title')}"></div>
            <span class="og-required-label">${t('config.webhooks.og_required_short')}</span>
        </div>
        <button class="wh-btn-icon wh-btn-danger" onclick="this.parentElement.remove()">\u2715</button>
    </div>`;
}

function ogAddParam() {
    const list = document.getElementById('og-params-list');
    if (!list) return;
    list.insertAdjacentHTML('beforeend', ogParamRow(list.children.length, {}));
}

function ogToggleTemplate() {
    const s = document.getElementById('og-f-ptype');
    const t = document.getElementById('og-f-body');
    if (s && t) t.disabled = s.value !== 'custom';
}

function ogCollectModal() {
    const params = [];
    document.querySelectorAll('#og-params-list .og-param-row').forEach(row => {
        const name = row.querySelector('.og-p-name')?.value?.trim();
        if (!name) return;
        params.push({
            name, type: row.querySelector('.og-p-type')?.value || 'string',
            description: row.querySelector('.og-p-desc')?.value?.trim() || '',
            required: row.querySelector('.toggle')?.classList.contains('on') || false
        });
    });
    const headersRaw = document.getElementById('og-f-headers').value.trim();
    const headers = {};
    if (headersRaw) headersRaw.split('\n').forEach(l => {
        const i = l.indexOf(':');
        if (i > 0) headers[l.substring(0, i).trim()] = l.substring(i + 1).trim();
    });
    return {
        id: ogEditingIdx >= 0 ? ogWebhooks[ogEditingIdx].id : 'hook_' + Date.now(),
        name: document.getElementById('og-f-name').value.trim(),
        description: document.getElementById('og-f-desc').value.trim(),
        method: document.getElementById('og-f-method').value,
        url: document.getElementById('og-f-url').value.trim(),
        headers, parameters: params,
        payload_type: document.getElementById('og-f-ptype').value,
        body_template: document.getElementById('og-f-body').value
    };
}

async function ogSave() {
    const hook = ogCollectModal();
    if (!hook.name) return showToast(t('config.webhooks.og_name_required'), 'error');
    if (!hook.url) return showToast(t('config.webhooks.og_url_required'), 'error');
    const newList = [...ogWebhooks];
    if (ogEditingIdx >= 0) newList[ogEditingIdx] = hook; else newList.push(hook);
    try {
        const resp = await fetch('/api/outgoing-webhooks', {
            method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(newList)
        });
        if (!resp.ok) { const r = await resp.json(); showToast(r.error || t('config.webhooks.error'), 'error'); return; }
        ogWebhooks = newList;
        ogCloseModal();
        showToast(t('config.webhooks.og_saved'), 'success');
        const panel = document.getElementById('wh-panel-outgoing');
        if (panel) panel.innerHTML = ogRenderList();
    } catch (e) { showToast(e.message, 'error'); }
}

async function ogDelete(idx) {
    const name = ogWebhooks[idx]?.name || t('config.webhooks.og_unnamed');
    if (!(await showConfirm(t('config.webhooks.og_delete_confirm_title'), t('config.webhooks.og_delete_confirm') + name + '?'))) return;
    const newList = ogWebhooks.filter((_, i) => i !== idx);
    try {
        const resp = await fetch('/api/outgoing-webhooks', {
            method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(newList)
        });
        if (resp.ok) {
            ogWebhooks = newList;
            showToast(t('config.webhooks.og_deleted'), 'success');
            const panel = document.getElementById('wh-panel-outgoing');
            if (panel) panel.innerHTML = ogRenderList();
        }
    } catch (e) { showToast(e.message, 'error'); }
}
