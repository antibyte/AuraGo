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

        // This module also renders on the knowledge page, which has no config helpers: load the
        // shared icon/dialog helpers on demand, and size icons explicitly (the config icon CSS is page scoped).
        function secretsEnsureHelpers() {
            if (typeof window.cfgModal === 'function' && typeof window.cfgIcon === 'function') return Promise.resolve();
            const load = src => new Promise(resolve => {
                const script = document.createElement('script');
                script.src = src;
                script.onload = script.onerror = () => resolve();
                document.head.appendChild(script);
            });
            return load('/js/precision/icons.js').then(() => load('/js/config/icons.js')).then(() => load('/js/config/components.js'));
        }

        function secretsIcon(name) {
            return typeof window.cfgIcon === 'function' ? window.cfgIcon(name, { size: 16 }) : '';
        }

        async function secretsErrorText(resp) {
            const txt = (await resp.text().catch(() => '')).trim();
            return txt || resp.statusText || t('config.common.error');
        }

        async function secretsSubmit(key, value) {
            try {
                const resp = await fetch('/api/vault/secrets', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ key, value })
                });
                if (!resp.ok) return { ok: false, error: await secretsErrorText(resp) };
                try {
                    const reload = await fetch('/api/vault/secrets');
                    if (reload.ok) secretsCache = await reload.json();
                } catch (_) { /* saved; list refresh failed */ }
                secretsRenderTable();
                return { ok: true };
            } catch (e) {
                return { ok: false, error: e.message || t('config.common.error') };
            }
        }

        async function renderSecretsSection(section) {
            // Support rendering into a custom container (e.g. knowledge center)
            await secretsEnsureHelpers();
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

            // Vault availability: 'present' | 'absent' | 'unknown'. Trust the Config page only when it
            // already knows the vault exists; an unreadable status is never reported as "no vault".
            let vaultStatus = ((typeof vaultState === 'string' && vaultState === 'present') || (typeof vaultExists !== 'undefined' && vaultExists === true))
                ? 'present'
                : 'unknown';
            if (vaultStatus !== 'present') {
                try {
                    const vResp = await fetch('/api/vault/status');
                    if (vResp.ok && vResp.status !== 204) {
                        const data = await vResp.json();
                        if (data && typeof data.exists === 'boolean') vaultStatus = data.exists ? 'present' : 'absent';
                    }
                } catch (_) {}
            }

            if (vaultStatus === 'unknown') {
                const statusEl = document.getElementById('secrets-vault-status');
                cfgListState(statusEl, { state: 'error', title: t('config.secrets.vault_status_unknown'), retry: () => renderSecretsSection(section) });
                return;
            }

            if (vaultStatus === 'absent') {
                document.getElementById('secrets-vault-status').innerHTML = `
                    <div class="secrets-empty secrets-empty-warning">
                        <div class="secrets-empty-icon">${secretsIcon('alert')}</div>
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
                    cfgListState('secrets-main', { state: 'error', title: t('config.secrets.load_error'), text: txt, retry: () => renderSecretsSection(section) });
                    return;
                }
                secretsCache = await resp.json();
            } catch (e) {
                cfgListState('secrets-main', { state: 'error', title: t('config.secrets.load_error'), text: e.message, retry: () => renderSecretsSection(section) });
                return;
            }
            secretsRenderTable();
        }

        function secretsRenderTable() {
            const wrap = document.getElementById('secrets-main');
            if (!wrap) return;

            let html = `<div id="secrets-inline-error" class="secrets-error is-hidden" role="alert"></div>
            <div class="kc-panel-header">
                <div class="kc-search-row">
                    <div class="secrets-meta">
                    ${secretsCache.length} ${t('config.secrets.count')}
                    </div>
                    <button class="btn-save secrets-btn-small" onclick="secretsShowAddModal()">
                    ${secretsIcon('plus')} ${t('config.secrets.new_secret')}
                    </button>
                </div>
            </div>`;

            if (secretsCache.length === 0) {
                html += '<div id="secrets-list-state"></div>';
            } else {
                html += `<div class="kc-table-wrap"><table class="kc-table">
                    <thead><tr>
                        <th>${t('config.secrets.key_column')}</th>
                        <th class="secrets-actions-column">${t('config.secrets.actions')}</th>
                    </tr></thead>
                    <tbody>`;
                secretsCache.forEach((s, idx) => {
                    const isSystem = s.key.startsWith('egg_') || s.key.startsWith('nest_') || s.key.startsWith('dev-') || s.key.startsWith('egg_shared_');
                    const badge = isSystem ? cfgBadge('idle', t('config.secrets.system_badge')) : '';
                    html += `<tr>
                        <td class="kc-mono">${escapeAttr(s.key)}${badge}</td>
                        <td class="secrets-table-actions">
                            <div class="kc-actions secrets-actions">
                                <button class="btn btn-secondary btn-sm" onclick="secretsEdit(${idx})" title="${t('config.secrets.edit')}" aria-label="${escapeAttr(t('config.secrets.edit') + ': ' + s.key)}">${secretsIcon('edit')}</button>
                                ${cfgDeleteButton({ iconOnly: true, label: t('config.secrets.delete') + ': ' + s.key, attrs: { onclick: 'secretsDelete(' + idx + ')' } })}
                            </div>
                        </td>
                    </tr>`;
                });
                html += '</tbody></table></div>';
            }
            wrap.innerHTML = html;
            if (secretsCache.length === 0) cfgListState('secrets-list-state', { state: 'empty', title: t('config.secrets.empty') });
        }

        const SECRETS_DIALOG = 'secrets-dialog';

        function secretsShowModal(title, keyVal, valueVal, keyEditable, onSave) {
            const existing = document.getElementById(SECRETS_DIALOG);
            if (existing) existing.remove();

            const body = `
                <div class="field-group">
                    <div class="field-label">${t('config.secrets.key_label')}</div>
                    <div class="field-help">${t('config.secrets.key_help')}</div>
                    <input class="field-input${keyEditable ? '' : ' is-readonly'}" id="secret-key" value="${escapeAttr(keyVal)}" placeholder="my_secret_key" ${keyEditable ? '' : 'disabled'}>
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.secrets.value_label')}</div>
                    <div class="field-help">${t('config.secrets.value_help')}</div>
                    <div class="password-wrap">
                        <input class="field-input" id="secret-value" type="password" placeholder="${t('config.secrets.value_placeholder')}" autocomplete="off">
                        <button type="button" class="password-toggle" data-visible="false" title="${escapeAttr(t('config.secrets.toggle_visibility'))}" aria-label="${escapeAttr(t('config.secrets.toggle_visibility'))}" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
                    </div>
                </div>
                <div id="secret-dialog-error" class="secrets-error is-hidden" role="alert"></div>`;
            const actions = [
                `<button type="button" class="btn btn-secondary" onclick="closeModal('${SECRETS_DIALOG}')">${t('config.secrets.cancel')}</button>`,
                `<button type="button" class="btn-save" id="secret-save-btn">${t('config.secrets.save')}</button>`
            ];
            document.body.insertAdjacentHTML('beforeend', cfgModal({ id: SECRETS_DIALOG, title, size: 'sm', body, actions }));
            const dialog = document.getElementById(SECRETS_DIALOG);

            document.getElementById('secret-save-btn').onclick = async () => {
                const errBox = document.getElementById('secret-dialog-error');
                const saveBtn = document.getElementById('secret-save-btn');
                const showErr = msg => { errBox.textContent = msg; errBox.classList.remove('is-hidden'); };
                errBox.classList.add('is-hidden');
                const key = document.getElementById('secret-key').value.trim();
                const value = document.getElementById('secret-value').value;
                if (!key) { showErr(t('config.secrets.key_empty')); return; }
                if (!value) { showErr(t('config.secrets.value_empty')); return; }
                saveBtn.disabled = true;
                const res = await onSave(key, value);
                saveBtn.disabled = false;
                if (res && res.ok === false) {
                    showErr(res.error || t('config.common.error'));
                    return;
                }
                closeModal(SECRETS_DIALOG);
            };

            openModal(SECRETS_DIALOG, { initialFocus: keyEditable ? '#secret-key' : '#secret-value', onClose: () => dialog.remove() });
        }

        function secretsShowAddModal() {
            secretsShowModal(
                t('config.secrets.new_secret'),
                '', '', true,
                (key, value) => secretsSubmit(key, value)
            );
        }

        function secretsEdit(idx) {
            const s = secretsCache[idx];
            secretsShowModal(
                t('config.secrets.edit_secret'),
                s.key, '', false,
                (key, value) => secretsSubmit(key, value)
            );
        }

        async function secretsDelete(idx) {
            const s = secretsCache[idx];
            const answer = await showConfirmDialog({
                title: t('config.secrets.delete_confirm_title'),
                message: t('config.secrets.delete_confirm', {key: s.key}),
                danger: true,
                confirmText: t('config.secrets.delete')
            });
            if (!answer.confirmed) return;

            const actionBtns = document.querySelectorAll('#secrets-main .secrets-actions button');
            actionBtns.forEach(b => { b.disabled = true; });
            let error = '';
            try {
                const resp = await fetch('/api/vault/secrets?key=' + encodeURIComponent(s.key), { method: 'DELETE' });
                if (!resp.ok) {
                    error = await secretsErrorText(resp);
                } else {
                    try {
                        const reload = await fetch('/api/vault/secrets');
                        if (reload.ok) secretsCache = await reload.json();
                    } catch (_) { /* deleted; list refresh failed */ }
                    secretsRenderTable();
                    return;
                }
            } catch (e) {
                error = e.message || t('config.common.error');
            }
            actionBtns.forEach(b => { b.disabled = false; });
            const box = document.getElementById('secrets-inline-error');
            if (box) { box.textContent = error; box.classList.remove('is-hidden'); }
            else showToast(error, 'error');
        }
