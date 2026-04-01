// cfg/email.js — Email Accounts Management section module

let emailAccountsCache = null;

        async function renderEmailSection(section) {
            // Lazy-load email accounts on first render
            if (emailAccountsCache === null) {
                try {
                    const resp = await fetch('/api/email-accounts');
                    emailAccountsCache = resp.ok ? await resp.json() : [];
                } catch (_) { emailAccountsCache = []; }
            }
            let html = `<div class="cfg-section active">
                <div class="section-header">${section.icon} ${section.label}</div>
                <div class="section-desc">${section.desc}</div>`;

            html += `<div style="display:flex;justify-content:flex-end;margin-bottom:1rem;">
                    <button class="btn-save" style="padding:0.45rem 1.1rem;font-size:0.82rem;" onclick="emailAccountAdd()">
                        ＋ ${t('config.email.new_account')}
                    </button>
                </div>
                <div id="email-accounts-list"></div>
                <div id="email-accounts-empty" style="display:none;text-align:center;padding:2rem;color:var(--text-tertiary);font-size:0.85rem;">
                    ${t('config.email.empty')}
                </div>
            </div>`;
            document.getElementById('content').innerHTML = html;
            emailAccountRenderCards();
        }

        function emailAccountRenderCards() {
            const wrap = document.getElementById('email-accounts-list');
            const empty = document.getElementById('email-accounts-empty');
            if (!wrap) return;
            if (emailAccountsCache.length === 0) {
                wrap.innerHTML = '';
                if (empty) empty.style.display = '';
                return;
            }
            if (empty) empty.style.display = 'none';

            let html = '';
            emailAccountsCache.forEach((a, idx) => {
                const watchBadge = a.watch_enabled
                    ? `<span style="display:inline-block;padding:0.15rem 0.5rem;border-radius:6px;font-size:0.7rem;font-weight:600;background:var(--success);color:#fff;margin-left:0.4rem;">👁️ Watcher</span>`
                    : '';
                const enabledBadge = (a.enabled === false)
                    ? `<span style="display:inline-block;padding:0.15rem 0.5rem;border-radius:6px;font-size:0.7rem;font-weight:600;background:var(--danger,#e55);color:#fff;margin-left:0.4rem;">⏸ ${t('config.email.disabled')}</span>`
                    : `<span style="display:inline-block;padding:0.15rem 0.5rem;border-radius:6px;font-size:0.7rem;font-weight:600;background:var(--success);color:#fff;margin-left:0.4rem;">✔ ${t('config.email.enabled')}</span>`;
                const sendBadge = (a.allow_sending === false)
                    ? `<span style="display:inline-block;padding:0.15rem 0.5rem;border-radius:6px;font-size:0.7rem;font-weight:600;background:var(--warning,#f90);color:#000;margin-left:0.4rem;">📖 ${t('config.email.read_only')}</span>`
                    : '';
                const maskedPw = a.password === '••••••••'
                    ? `<span style="color:var(--success);">✔ ${t('config.email.password_set')}</span>`
                    : (a.password ? `<span style="color:var(--success);">✔ ${t('config.email.password_set')}</span>` : '<span style="color:var(--text-tertiary);">—</span>');

                html += `
                <div class="provider-card" data-idx="${idx}" style="border:1px solid var(--border-subtle);border-radius:12px;padding:1rem 1.2rem;margin-bottom:0.75rem;background:var(--bg-secondary);transition:border-color 0.15s;">
                    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:0.6rem;">
                        <div style="font-weight:600;font-size:0.9rem;">
                            ${escapeAttr(a.name || a.id)}${enabledBadge}${sendBadge}${watchBadge}
                            <span style="font-size:0.72rem;color:var(--text-tertiary);margin-left:0.5rem;">ID: ${escapeAttr(a.id)}</span>
                        </div>
                        <div style="display:flex;gap:0.4rem;">
                            <button onclick="emailAccountEdit(${idx})" style="background:none;border:none;cursor:pointer;color:var(--accent);font-size:0.85rem;" title="Edit">✏️</button>
                            <button onclick="emailAccountDelete(${idx})" style="background:none;border:none;cursor:pointer;color:var(--danger);font-size:0.85rem;" title="Delete">🗑️</button>
                        </div>
                    </div>
                    <div style="display:grid;grid-template-columns:1fr 1fr;gap:0.3rem 1rem;font-size:0.78rem;">
                        <div><span style="color:var(--text-tertiary);">IMAP:</span> ${escapeAttr(a.imap_host || '—')}:${a.imap_port || '—'}</div>
                        <div><span style="color:var(--text-tertiary);">SMTP:</span> ${escapeAttr(a.smtp_host || '—')}:${a.smtp_port || '—'}</div>
                        <div><span style="color:var(--text-tertiary);">${t('config.email.user')}:</span> ${escapeAttr(a.username || '—')}</div>
                        <div><span style="color:var(--text-tertiary);">${t('config.email.password')}:</span> ${maskedPw}</div>
                        <div><span style="color:var(--text-tertiary);">From:</span> ${escapeAttr(a.from_address || '—')}</div>
                        <div><span style="color:var(--text-tertiary);">${t('config.email.folder')}:</span> ${escapeAttr(a.watch_folder || 'INBOX')}</div>
                    </div>
                </div>`;
            });
            wrap.innerHTML = html;
        }

        function emailAccountShowModal(title, data, onSave) {
            const existing = document.getElementById('email-modal-overlay');
            if (existing) existing.remove();

            const overlay = document.createElement('div');
            overlay.id = 'email-modal-overlay';
            overlay.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,0.55);z-index:1000;backdrop-filter:blur(4px);display:flex;align-items:center;justify-content:center;';
            overlay.onclick = (e) => { if (e.target === overlay) overlay.remove(); };

            overlay.innerHTML = `
            <div style="background:var(--bg-secondary);border-radius:16px;padding:1.5rem;width:min(520px,92vw);max-height:85vh;overflow-y:auto;border:1px solid var(--border-subtle);" onclick="event.stopPropagation()">
                <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:1.2rem;">
                    <div style="font-weight:700;font-size:1rem;">${title}</div>
                    <button onclick="document.getElementById('email-modal-overlay').remove()" style="background:none;border:none;color:var(--text-secondary);font-size:1.2rem;cursor:pointer;">✕</button>
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.email.field_id')}</div>
                    <div class="field-help">${t('config.email.id_help')}</div>
                    <input class="field-input" id="ea-id" value="${escapeAttr(data.id || '')}" placeholder="personal" ${data._editMode ? 'disabled style="opacity:0.55;cursor:not-allowed;"' : ''}>
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.email.field_name')}</div>
                    <input class="field-input" id="ea-name" value="${escapeAttr(data.name || '')}" placeholder="${t('config.email.display_name')}">
                </div>

                <div style="margin-top:0.8rem;padding-top:0.8rem;border-top:1px solid var(--border-subtle);">
                    <div style="font-weight:600;font-size:0.85rem;color:var(--accent);margin-bottom:0.6rem;">⚙️ ${t('config.email.account_settings')}</div>
                </div>
                <div class="field-group" style="display:flex;align-items:center;gap:0.8rem;">
                    <div style="flex:1;">
                        <div class="field-label" style="margin-bottom:0.1rem;">${t('config.email.account_enabled_label')}</div>
                        <div class="field-help" style="margin:0;">${t('config.email.account_enabled_help')}</div>
                    </div>
                    <label class="toggle-switch">
                        <input type="checkbox" id="ea-enabled" ${(data.enabled !== false) ? 'checked' : ''}>
                        <span class="toggle-slider"></span>
                    </label>
                </div>
                <div class="field-group" style="display:flex;align-items:center;gap:0.8rem;">
                    <div style="flex:1;">
                        <div class="field-label" style="margin-bottom:0.1rem;">${t('config.email.allow_sending_label')}</div>
                        <div class="field-help" style="margin:0;">${t('config.email.allow_sending_help')}</div>
                    </div>
                    <label class="toggle-switch">
                        <input type="checkbox" id="ea-allow-sending" ${(data.allow_sending !== false) ? 'checked' : ''}>
                        <span class="toggle-slider"></span>
                    </label>
                </div>

                <div style="margin-top:0.8rem;padding-top:0.8rem;border-top:1px solid var(--border-subtle);">
                    <div style="font-weight:600;font-size:0.85rem;color:var(--accent);margin-bottom:0.6rem;">📥 ${t('config.email.imap_receive')}</div>
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.email.imap_host')}</div>
                    <input class="field-input" id="ea-imap-host" value="${escapeAttr(data.imap_host || '')}" placeholder="imap.example.com">
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.email.imap_port')}</div>
                    <div class="field-help">${t('config.email.imap_port_help')}</div>
                    <input class="field-input" id="ea-imap-port" type="number" value="${data.imap_port || 993}" placeholder="993">
                </div>

                <div style="margin-top:0.8rem;padding-top:0.8rem;border-top:1px solid var(--border-subtle);">
                    <div style="font-weight:600;font-size:0.85rem;color:var(--accent);margin-bottom:0.6rem;">📤 ${t('config.email.smtp_send')}</div>
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.email.smtp_host')}</div>
                    <input class="field-input" id="ea-smtp-host" value="${escapeAttr(data.smtp_host || '')}" placeholder="smtp.example.com">
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.email.smtp_port')}</div>
                    <div class="field-help">${t('config.email.smtp_port_help')}</div>
                    <input class="field-input" id="ea-smtp-port" type="number" value="${data.smtp_port || 587}" placeholder="587">
                </div>

                <div style="margin-top:0.8rem;padding-top:0.8rem;border-top:1px solid var(--border-subtle);">
                    <div style="font-weight:600;font-size:0.85rem;color:var(--accent);margin-bottom:0.6rem;">🔑 ${t('config.email.credentials')}</div>
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.email.username')}</div>
                    <input class="field-input" id="ea-username" value="${escapeAttr(data.username || '')}" placeholder="user@example.com">
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.email.password')}</div>
                    <div class="password-wrap">
                        <input class="field-input" id="ea-password" type="password" value="${escapeAttr(data.password === '••••••••' ? '' : (data.password || ''))}" placeholder="${data.password === '••••••••' ? '••••••••  (' + t('config.email.password_unchanged') + ')' : ''}" autocomplete="off">
                        <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
                    </div>
                    ${data.password === '••••••••' ? `<div style="font-size:0.72rem;color:var(--text-tertiary);margin-top:0.2rem;">${t('config.email.keep_password')}</div>` : ''}
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.email.from_address')}</div>
                    <div class="field-help">${t('config.email.from_help')}</div>
                    <input class="field-input" id="ea-from" value="${escapeAttr(data.from_address || '')}" placeholder="user@example.com">
                </div>

                <div style="margin-top:0.8rem;padding-top:0.8rem;border-top:1px solid var(--border-subtle);">
                    <div style="font-weight:600;font-size:0.85rem;color:var(--accent);margin-bottom:0.6rem;">👁️ ${t('config.email.inbox_watcher')}</div>
                </div>
                <div class="field-group" style="display:flex;align-items:center;gap:0.8rem;">
                    <div class="field-label" style="margin-bottom:0;">${t('config.email.watch_enabled')}</div>
                    <label class="toggle-switch">
                        <input type="checkbox" id="ea-watch-enabled" ${data.watch_enabled ? 'checked' : ''}>
                        <span class="toggle-slider"></span>
                    </label>
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.email.watch_folder')}</div>
                    <input class="field-input" id="ea-watch-folder" value="${escapeAttr(data.watch_folder || 'INBOX')}" placeholder="INBOX">
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.email.interval_seconds')}</div>
                    <input class="field-input" id="ea-watch-interval" type="number" value="${data.watch_interval_seconds || 120}" placeholder="120">
                </div>

                <div style="display:flex;justify-content:flex-end;gap:0.6rem;margin-top:1.2rem;">
                    <button class="btn-save" style="padding:0.45rem 1.4rem;font-size:0.82rem;background:var(--bg-tertiary);color:var(--text-primary);" onclick="document.getElementById('email-modal-overlay').remove()">
                        ${t('config.email.cancel')}
                    </button>
                    <button class="btn-save" style="padding:0.45rem 1.4rem;font-size:0.82rem;" id="ea-save-btn">
                        ${t('config.email.save')}
                    </button>
                </div>
            </div>`;
            document.body.appendChild(overlay);

            // Save handler
            document.getElementById('ea-save-btn').onclick = () => {
                const id = document.getElementById('ea-id').value.trim().toLowerCase().replace(/[^a-z0-9_-]/g, '');
                const name = document.getElementById('ea-name').value.trim();
                const enabled = document.getElementById('ea-enabled').checked;
                const allow_sending = document.getElementById('ea-allow-sending').checked;
                const imap_host = document.getElementById('ea-imap-host').value.trim();
                const imap_port = parseInt(document.getElementById('ea-imap-port').value, 10) || 993;
                const smtp_host = document.getElementById('ea-smtp-host').value.trim();
                const smtp_port = parseInt(document.getElementById('ea-smtp-port').value, 10) || 587;
                const username = document.getElementById('ea-username').value.trim();
                let password = document.getElementById('ea-password').value.trim();
                if (!password && data.password === '••••••••') password = '••••••••';
                const from_address = document.getElementById('ea-from').value.trim();
                const watch_enabled = document.getElementById('ea-watch-enabled').checked;
                const watch_folder = document.getElementById('ea-watch-folder').value.trim() || 'INBOX';
                const watch_interval_seconds = parseInt(document.getElementById('ea-watch-interval').value, 10) || 120;

                if (!id) { showToast(t('config.email.id_empty'), 'warn'); return; }
                if (!imap_host && !smtp_host) { showToast(t('config.email.host_required'), 'warn'); return; }

                const entry = { id, name: name || id, enabled, allow_sending, imap_host, imap_port, smtp_host, smtp_port, username, password, from_address, watch_enabled, watch_folder, watch_interval_seconds };
                onSave(entry);
                overlay.remove();
            };

            // Focus first editable field
            setTimeout(() => {
                const focus = data._editMode ? document.getElementById('ea-name') : document.getElementById('ea-id');
                if (focus) focus.focus();
            }, 50);
        }

        function emailAccountAdd() {
            emailAccountShowModal(
                t('config.email.new_account'),
                {},
                async (entry) => {
                    if (emailAccountsCache.some(a => a.id === entry.id)) {
                        showToast(t('config.email.id_exists'), 'warn');
                        return;
                    }
                    emailAccountsCache.push(entry);
                    await emailAccountSave();
                }
            );
        }

        function emailAccountEdit(idx) {
            const a = { ...emailAccountsCache[idx], _editMode: true };
            emailAccountShowModal(
                t('config.email.edit_account'),
                a,
                async (entry) => {
                    emailAccountsCache[idx] = entry;
                    await emailAccountSave();
                }
            );
        }

        function emailAccountDelete(idx) {
            const a = emailAccountsCache[idx];
            if (!confirm(t('config.email.delete_confirm', {name: a.name || a.id}))) return;
            emailAccountsCache.splice(idx, 1);
            emailAccountSave();
        }

        async function emailAccountSave() {
            try {
                const resp = await fetch('/api/email-accounts', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(emailAccountsCache)
                });
                if (!resp.ok) {
                    const txt = await resp.text();
                    showToast(txt || t('config.common.error'), 'error');
                    return;
                }
                // Reload from server (passwords will be masked)
                const reload = await fetch('/api/email-accounts');
                if (reload.ok) emailAccountsCache = await reload.json();
                emailAccountRenderCards();
            } catch (e) {
                showToast(e.message || t('config.common.error'), 'error');
            }
        }
