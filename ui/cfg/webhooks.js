// cfg/webhooks.js — Webhooks, Tokens & Log section module

let whWebhooks = [];
let whTokens = [];
let whPresets = [];
let whLog = [];
let whEditingId = null; // null = new, string = editing

        async function whFetchAll() {
            try {
                const [wResp, tResp, pResp] = await Promise.all([
                    fetch('/api/webhooks'), fetch('/api/tokens'), fetch('/api/webhooks/presets')
                ]);
                if (wResp.ok) whWebhooks = await wResp.json();
                if (tResp.ok) whTokens = await tResp.json();
                if (pResp.ok) whPresets = await pResp.json();
            } catch (e) { console.error('webhook fetch error', e); }
        }

        async function renderWebhooksSection(section) {
            const content = document.getElementById('content');
            content.innerHTML = '<div class="cfg-section active"><div style="text-align:center;padding:3rem;color:var(--text-secondary);">' + t('config.webhooks.loading') + '</div></div>';
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

            html += '</div>';
            content.innerHTML = html;

            // Load log if enabled
            if (enabled) whLoadLog();
        }

        function whRenderConfigSettings() {
            const wh = configData.webhooks || {};
            return `
            <div style="margin-top:1.5rem;margin-bottom:1rem;">
                <div style="font-weight:600;font-size:0.85rem;color:var(--accent);margin-bottom:0.75rem;border-bottom:1px solid var(--border-subtle);padding-bottom:0.3rem;">
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
                <span class="wh-count">${whWebhooks.length} / ${10} Webhooks</span>
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
                                <button class="wh-btn-icon" title="${t('config.webhooks.action_test')}" onclick="whTestWebhook('${w.id}')">🧪</button>
                                <button class="wh-btn-icon" title="${t('config.webhooks.action_edit')}" onclick="whShowEditor('${w.id}')">✏️</button>
                                <button class="wh-btn-icon wh-btn-danger" title="${t('config.webhooks.action_delete')}" onclick="whDeleteWebhook('${w.id}','${esc(w.name)}')">🗑️</button>
                            </div>
                        </div>
                        <div class="wh-card-body">
                            <div class="wh-card-url" onclick="whCopy(this, '${esc(url)}')" title="${t('config.webhooks.click_to_copy')}">
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
            html += '<div id="wh-editor" class="wh-editor" style="display:none;"></div>';
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
                </div>
                <div class="wh-form-row">
                    <label>${t('config.webhooks.preset_label')}</label>
                    <select id="wh-f-preset" class="field-select" onchange="whApplyPreset(this.value)">${presetsHtml}</select>
                </div>
                <div style="font-weight:600;font-size:0.8rem;color:var(--accent);margin-top:1rem;margin-bottom:0.5rem;">${t('config.webhooks.format_section')}</div>
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
                        <option value="sha256" ${format.signature_algo === 'sha256' ? 'selected' : ''}>sha256</option>
                        <option value="sha1" ${format.signature_algo === 'sha1' ? 'selected' : ''}>sha1</option>
                        <option value="plain" ${format.signature_algo === 'plain' ? 'selected' : ''}>plain</option>
                    </select>
                </div>
                <div class="wh-form-row">
                    <label>${t('config.webhooks.signature_secret_label')}</label>
                    <input id="wh-f-sigsec" class="field-input" type="password" value="${esc(format.signature_secret || '')}" placeholder="${t('config.webhooks.sig_secret_placeholder')}">
                </div>
                <div style="font-weight:600;font-size:0.8rem;color:var(--accent);margin-top:1rem;margin-bottom:0.5rem;">
                    ${t('config.webhooks.field_mappings_label')}
                    <button class="wh-btn wh-btn-sm" onclick="whAddFieldRow()" style="margin-left:0.5rem;">${t('config.webhooks.add_field_button')}</button>
                </div>
                <div id="wh-fields-list">${fieldsHtml}</div>
                <div style="font-weight:600;font-size:0.8rem;color:var(--accent);margin-top:1rem;margin-bottom:0.5rem;">${t('config.webhooks.delivery_section')}</div>
                <div class="wh-form-row">
                    <label>${t('config.webhooks.mode_label')}</label>
                    <select id="wh-f-mode" class="field-select">
                        <option value="message" ${delivery.mode === 'message' ? 'selected' : ''}>Message (${t('config.webhooks.mode_message')})</option>
                        <option value="notify" ${delivery.mode === 'notify' ? 'selected' : ''}>${t('config.webhooks.mode_notify')}</option>
                        <option value="silent" ${delivery.mode === 'silent' ? 'selected' : ''}>Silent (${t('config.webhooks.mode_silent')})</option>
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
            ed.style.display = 'block';
            ed.scrollIntoView({ behavior: 'smooth', block: 'start' });
        }

        function whHideEditor() {
            const ed = document.getElementById('wh-editor');
            if (ed) ed.style.display = 'none';
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
            if (!data.name) return whToast(t('config.webhooks.name_required'), 'error');
            if (!whEditingId && !data.slug) return whToast(t('config.webhooks.slug_required'), 'error');

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
                    whToast(result.error || 'Error', 'error');
                    return;
                }
                whToast(t('config.webhooks.saved'), 'success');
                whHideEditor();
                // Refresh
                const section = SECTIONS.flatMap(g => g.items).find(s => s.key === 'webhooks');
                await renderWebhooksSection(section);
            } catch (e) {
                whToast(e.message, 'error');
            }
        }

        async function whDeleteWebhook(id, name) {
            if (!confirm(t('config.webhooks.delete_confirm') + name + '?')) return;
            try {
                const resp = await fetch('/api/webhooks/' + id, { method: 'DELETE' });
                if (resp.ok) {
                    whToast(t('config.webhooks.deleted'), 'success');
                    const section = SECTIONS.flatMap(g => g.items).find(s => s.key === 'webhooks');
                    await renderWebhooksSection(section);
                } else {
                    const r = await resp.json();
                    whToast(r.error || 'Error', 'error');
                }
            } catch (e) { whToast(e.message, 'error'); }
        }

        async function whTestWebhook(id) {
            try {
                const resp = await fetch('/api/webhooks/' + id + '/test', { method: 'POST' });
                const r = await resp.json();
                if (resp.ok) {
                    whToast(t('config.webhooks.test_prefix') + (r.prompt || 'OK').substring(0, 100), 'success');
                } else {
                    whToast(r.error || t('config.webhooks.test_failed'), 'error');
                }
            } catch (e) { whToast(e.message, 'error'); }
        }

        /* ── Token Management ── */
        function whRenderTokenList() {
            let html = `<div class="wh-toolbar">
                <span class="wh-count">${whTokens.length} Tokens</span>
                <button class="wh-btn wh-btn-primary" onclick="whCreateToken()">+ ${t('config.tokens.new_token')}</button>
            </div>`;
            html += '<div id="wh-token-created" style="display:none;" class="wh-token-reveal"></div>';

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
                        <div class="wh-card-meta" style="margin-top:0.4rem;">
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
                if (!resp.ok) { whToast(r.error || 'Error', 'error'); return; }
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
            } catch (e) { whToast(e.message, 'error'); }
        }

        async function whToggleToken(id, enabled) {
            try {
                const resp = await fetch('/api/tokens/' + id, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ enabled: enabled })
                });
                if (resp.ok) {
                    whToast(enabled ? t('config.tokens.activated') : t('config.tokens.deactivated'), 'success');
                    const tResp = await fetch('/api/tokens');
                    if (tResp.ok) whTokens = await tResp.json();
                    const panel = document.getElementById('wh-panel-tokens');
                    if (panel) panel.innerHTML = whRenderTokenList();
                }
            } catch (e) { whToast(e.message, 'error'); }
        }

        async function whDeleteToken(id, name) {
            if (!confirm(t('config.tokens.delete_confirm') + name + '?')) return;
            try {
                const resp = await fetch('/api/tokens/' + id, { method: 'DELETE' });
                if (resp.ok) {
                    whToast(t('config.tokens.deleted'), 'success');
                    const tResp = await fetch('/api/tokens');
                    if (tResp.ok) whTokens = await tResp.json();
                    const panel = document.getElementById('wh-panel-tokens');
                    if (panel) panel.innerHTML = whRenderTokenList();
                }
            } catch (e) { whToast(e.message, 'error'); }
        }

        /* ── Webhook Log ── */
        function whRenderLog() {
            return `<div id="wh-log-content" class="wh-log-content">
                <div style="text-align:center;padding:1rem;color:var(--text-secondary);">${t('config.webhooks.log_loading')}</div>
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
                        <td>${e.payload_size || 0} B</td>
                        <td>${e.delivered ? '✓' : (e.error ? '✗ ' + esc(e.error) : '—')}</td>
                    </tr>`;
                }
                html += '</tbody></table>';
                el.innerHTML = html;
            } catch (e) {
                const el = document.getElementById('wh-log-content');
                if (el) el.innerHTML = '<div class="wh-empty" style="color:var(--danger);">' + t('config.webhooks.log_error_prefix') + esc(e.message) + '</div>';
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

        function whToast(msg, type) {
            const toast = document.createElement('div');
            toast.className = 'wh-toast ' + (type || '');
            toast.textContent = msg;
            document.body.appendChild(toast);
            setTimeout(() => { toast.classList.add('wh-toast-exit'); setTimeout(() => toast.remove(), 300); }, 3000);
        }
