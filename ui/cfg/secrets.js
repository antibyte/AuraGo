// cfg/secrets.js — Secrets / Vault Management section module

        // Fallbacks for globals defined in config/main.js (not available in other pages)
        if (typeof window.EYE_OPEN_SVG === 'undefined') {
            window.EYE_OPEN_SVG = '<svg viewBox="0 0 24 24"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>';
        }
        if (typeof window.EYE_CLOSED_SVG === 'undefined') {
            window.EYE_CLOSED_SVG = '<svg viewBox="0 0 24 24"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>';
        }
        if (typeof window.escapeAttr === 'undefined') {
            window.escapeAttr = function(s) { return String(s).replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;'); };
        }
        if (typeof window.togglePassword === 'undefined') {
            window.togglePassword = function(btn) {
                const wrap = btn.closest('.password-wrap');
                const inp = wrap && wrap.querySelector('input');
                if (!inp) return;
                const show = btn.getAttribute('data-visible') !== 'true';
                btn.setAttribute('data-visible', show ? 'true' : 'false');
                inp.type = show ? 'text' : 'password';
                btn.innerHTML = show ? window.EYE_CLOSED_SVG : window.EYE_OPEN_SVG;
            };
        }

        let secretsCache = [];

        async function renderSecretsSection(section) {
            // Support rendering into a custom container (e.g. knowledge center)
            const containerId = section.container || 'content';
            const content = document.getElementById(containerId);
            if (!content) return;

            if (section.container) {
                // Knowledge center context: render without cfg-specific wrapper
                content.innerHTML = `
                <div class="secrets-panel-pad">
                    <div id="secrets-vault-status" class="secrets-status"></div>
                    <div id="secrets-main"></div>
                </div>`;
            } else {
                content.innerHTML = `<div class="cfg-section active">
                <div class="section-header">${section.label}</div>
                <div class="section-desc">${section.desc}</div>
                <div id="secrets-vault-status" class="secrets-status"></div>
                <div id="secrets-main"></div>
            </div>`;
            }

            // Check vault availability — use global if set, otherwise fetch
            let vaultReady = typeof vaultExists !== 'undefined' ? vaultExists : false;
            if (!vaultReady) {
                try {
                    const vResp = await fetch('/api/vault/status');
                    if (vResp.ok && vResp.status !== 204) { vaultReady = (await vResp.json()).exists === true; }
                } catch (_) {}
            }

            if (!vaultReady) {
                document.getElementById('secrets-vault-status').innerHTML = `
                    <div class="secrets-empty secrets-empty-warning">
                        <div class="secrets-empty-icon">⚠️</div>
                        <div class="secrets-empty-title">${t('config.secrets.no_vault')}</div>
                        <div class="secrets-meta secrets-empty-desc">
                            ${t('config.secrets.no_vault_desc')}
                        </div>
                    </div>`;
                return;
            }

            // Load secrets
            try {
                const resp = await fetch('/api/vault/secrets');
                if (!resp.ok) {
                    const txt = await resp.text();
                    document.getElementById('secrets-main').innerHTML = `
                        <div class="secrets-error">
                            ❌ ${t('config.secrets.load_error')}: ${txt}
                        </div>`;
                    return;
                }
                secretsCache = await resp.json();
            } catch (e) {
                document.getElementById('secrets-main').innerHTML = `
                    <div class="secrets-error">❌ ${e.message}</div>`;
                return;
            }
            secretsRenderTable();
        }

        function secretsRenderTable() {
            const wrap = document.getElementById('secrets-main');
            if (!wrap) return;

            let html = `<div class="kc-panel-header">
                <div class="kc-search-row">
                    <div class="secrets-meta">
                    ${secretsCache.length} ${t('config.secrets.count')}
                    </div>
                    <button class="btn-save secrets-btn-small" onclick="secretsShowAddModal()">
                    ＋ ${t('config.secrets.new_secret')}
                    </button>
                </div>
            </div>`;

            if (secretsCache.length === 0) {
                html += `<div class="secrets-empty secrets-empty-neutral">
                    ${t('config.secrets.empty')}
                </div>`;
            } else {
                html += `<div class="kc-table-wrap"><table class="kc-table">
                    <thead><tr>
                        <th>Key</th>
                        <th class="secrets-actions-column">${t('config.secrets.actions')}</th>
                    </tr></thead>
                    <tbody>`;
                secretsCache.forEach((s, idx) => {
                    const isSystem = s.key.startsWith('egg_') || s.key.startsWith('nest_') || s.key.startsWith('dev-') || s.key.startsWith('egg_shared_');
                    const badge = isSystem ? `<span class="secrets-system-badge">system</span>` : '';
                    html += `<tr>
                        <td class="kc-mono">${escapeAttr(s.key)}${badge}</td>
                        <td class="secrets-table-actions">
                            <div class="kc-actions secrets-actions">
                                <button class="btn btn-secondary btn-sm" onclick="secretsEdit(${idx})" title="${t('config.secrets.edit')}">✏️</button>
                                <button class="btn btn-danger btn-sm" onclick="secretsDelete(${idx})" title="${t('config.secrets.delete')}">🗑️</button>
                            </div>
                        </td>
                    </tr>`;
                });
                html += '</tbody></table></div>';
            }
            wrap.innerHTML = html;
        }

        function secretsShowModal(title, keyVal, valueVal, keyEditable, onSave) {
            const existing = document.getElementById('secrets-modal-overlay');
            if (existing) existing.remove();

            const overlay = document.createElement('div');
            overlay.id = 'secrets-modal-overlay';
            overlay.className = 'modal-overlay open active';
            overlay.onclick = (e) => { if (e.target === overlay) overlay.remove(); };

            overlay.innerHTML = `
            <div class="modal-card secrets-modal-card" onclick="event.stopPropagation()">
                <div class="modal-header">
                    <h2>${title}</h2>
                    <button type="button" class="modal-close" onclick="document.getElementById('secrets-modal-overlay').remove()" aria-label="${t('config.secrets.cancel')}">&times;</button>
                </div>
                <div class="field-group">
                    <div class="field-label">Key</div>
                    <div class="field-help">${t('config.secrets.key_help')}</div>
                    <input class="field-input${keyEditable ? '' : ' is-readonly'}" id="secret-key" value="${escapeAttr(keyVal)}" placeholder="my_secret_key" ${keyEditable ? '' : 'disabled'}>
                </div>
                <div class="field-group">
                    <div class="field-label">Value</div>
                    <div class="field-help">${t('config.secrets.value_help')}</div>
                    <div class="password-wrap">
                        <input class="field-input" id="secret-value" type="password" placeholder="${t('config.secrets.value_placeholder')}" autocomplete="off">
                        <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
                    </div>
                </div>
                <div class="modal-actions">
                    <button type="button" class="btn btn-secondary" onclick="document.getElementById('secrets-modal-overlay').remove()">
                        ${t('config.secrets.cancel')}
                    </button>
                    <button type="button" class="btn-save" id="secret-save-btn">
                        ${t('config.secrets.save')}
                    </button>
                </div>
            </div>`;
            document.body.appendChild(overlay);

            document.getElementById('secret-save-btn').onclick = () => {
                const key = document.getElementById('secret-key').value.trim();
                const value = document.getElementById('secret-value').value;
                if (!key) { showToast(t('config.secrets.key_empty'), 'warn'); return; }
                if (!value) { showToast(t('config.secrets.value_empty'), 'warn'); return; }
                onSave(key, value);
                overlay.remove();
            };

            setTimeout(() => {
                const focus = keyEditable ? document.getElementById('secret-key') : document.getElementById('secret-value');
                if (focus) focus.focus();
            }, 50);
        }

        function secretsShowAddModal() {
            secretsShowModal(
                t('config.secrets.new_secret'),
                '', '', true,
                async (key, value) => {
                    try {
                        const resp = await fetch('/api/vault/secrets', {
                            method: 'POST',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify({ key, value })
                        });
                        if (!resp.ok) {
                            const txt = await resp.text();
                            showToast(txt || t('config.common.error'), 'error');
                            return;
                        }
                        // Reload
                        const reload = await fetch('/api/vault/secrets');
                        if (reload.ok) secretsCache = await reload.json();
                        secretsRenderTable();
                    } catch (e) {
                        showToast(e.message || t('config.common.error'), 'error');
                    }
                }
            );
        }

        function secretsEdit(idx) {
            const s = secretsCache[idx];
            secretsShowModal(
                t('config.secrets.edit_secret'),
                s.key, '', false,
                async (key, value) => {
                    try {
                        const resp = await fetch('/api/vault/secrets', {
                            method: 'POST',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify({ key, value })
                        });
                        if (!resp.ok) {
                            const txt = await resp.text();
                            showToast(txt || t('config.common.error'), 'error');
                            return;
                        }
                        // Reload
                        const reload = await fetch('/api/vault/secrets');
                        if (reload.ok) secretsCache = await reload.json();
                        secretsRenderTable();
                    } catch (e) {
                        showToast(e.message || t('config.common.error'), 'error');
                    }
                }
            );
        }

        async function secretsDelete(idx) {
            const s = secretsCache[idx];
            if (!(await showConfirm(t('config.secrets.delete_confirm_title'), t('config.secrets.delete_confirm', {key: s.key})))) return;

            try {
                const resp = await fetch('/api/vault/secrets?key=' + encodeURIComponent(s.key), { method: 'DELETE' });
                if (!resp.ok) {
                    const txt = await resp.text();
                    showToast(txt || t('config.common.error'), 'error');
                    return;
                }
                // Reload
                const reload = await fetch('/api/vault/secrets');
                if (reload.ok) secretsCache = await reload.json();
                secretsRenderTable();
            } catch (e) {
                showToast(e.message || t('config.common.error'), 'error');
            }
        }
