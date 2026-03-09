// cfg/backup.js — Backup & Restore section module

        function renderBackupSection(section) {
            let html = '<div class="cfg-section active">';
            html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
            html += '<div class="section-desc">' + section.desc + '</div>';

            html += `
            <!-- Tabs -->
            <div class="wh-tabs" style="margin-top:1.5rem;">
                <button class="wh-tab active" id="backup-tab-create" onclick="backupShowTab('create')">
                    ${t('config.backup.tab_create')}
                </button>
                <button class="wh-tab" id="backup-tab-import" onclick="backupShowTab('import')">
                    ${t('config.backup.tab_import')}
                </button>
            </div>

            <!-- Create panel -->
            <div class="wh-panel active" id="backup-panel-create" style="padding-top:1.25rem;">
                <div style="background:var(--bg-secondary);border:1px solid var(--border-subtle);border-radius:12px;padding:1.1rem 1.25rem;margin-bottom:1rem;">
                    <div style="font-size:0.8rem;color:var(--text-secondary);line-height:1.6;">
                        ${t('config.backup.create_desc')}
                    </div>
                </div>

                <div style="display:flex;flex-direction:column;gap:0.7rem;margin-bottom:1.25rem;">
                    <label style="display:flex;align-items:center;gap:0.65rem;cursor:pointer;font-size:0.85rem;">
                        <input type="checkbox" id="backup-opt-vectordb" style="accent-color:var(--accent);width:16px;height:16px;">
                        <span>${t('config.backup.include_vectordb')}</span>
                    </label>
                    <label style="display:flex;align-items:center;gap:0.65rem;cursor:pointer;font-size:0.85rem;">
                        <input type="checkbox" id="backup-opt-workdir" style="accent-color:var(--accent);width:16px;height:16px;">
                        <span>${t('config.backup.include_workdir')}</span>
                    </label>
                </div>

                <div class="field-group" style="margin-bottom:1.25rem;">
                    <div class="field-label">${t('config.backup.password_label')}</div>
                    <div class="field-hint" style="margin-bottom:0.5rem;">${t('config.backup.password_hint')}</div>
                    <div class="password-wrap">
                        <input type="password" id="backup-password" class="field-input" placeholder="${t('config.backup.encryption_placeholder')}" autocomplete="new-password">
                        <button type="button" class="password-toggle" onclick="(function(b){var i=document.getElementById('backup-password');i.type=i.type==='password'?'text':'password';b.innerHTML=i.type==='password'?EYE_OPEN_SVG:EYE_CLOSED_SVG;})(this)" title="Toggle visibility">${'<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="width:16px;height:16px;"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>'}</button>
                    </div>
                </div>

                <div style="display:flex;gap:0.75rem;align-items:center;flex-wrap:wrap;">
                    <button class="btn-save" style="padding:0.55rem 1.4rem;font-size:0.85rem;" onclick="backupCreate()" id="backup-create-btn">
                        ${t('config.backup.create_download')}
                    </button>
                    <span id="backup-create-status" style="font-size:0.8rem;color:var(--text-secondary);"></span>
                </div>
            </div>

            <!-- Import panel -->
            <div class="wh-panel" id="backup-panel-import" style="padding-top:1.25rem;">
                <div style="background:rgba(245,158,11,0.08);border:1px solid rgba(245,158,11,0.25);border-radius:12px;padding:1rem 1.25rem;margin-bottom:1.25rem;font-size:0.82rem;color:var(--warning);line-height:1.6;">
                    ⚠️ ${t('config.backup.import_warning')}
                </div>

                <div class="field-group" style="margin-bottom:1rem;">
                    <div class="field-label">${t('config.backup.select_file')}</div>
                    <input type="file" id="backup-import-file" accept=".ago" class="field-input" style="padding:0.4rem;cursor:pointer;">
                </div>

                <div class="field-group" style="margin-bottom:1.25rem;">
                    <div class="field-label">${t('config.backup.import_password_label')}</div>
                    <div class="password-wrap">
                        <input type="password" id="backup-import-password" class="field-input" placeholder="${t('config.backup.decryption_placeholder')}" autocomplete="off">
                        <button type="button" class="password-toggle" onclick="(function(b){var i=document.getElementById('backup-import-password');i.type=i.type==='password'?'text':'password';b.innerHTML=i.type==='password'?EYE_OPEN_SVG:EYE_CLOSED_SVG;})(this)" title="Toggle visibility">${'<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="width:16px;height:16px;"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>'}</button>
                    </div>
                </div>

                <div style="display:flex;gap:0.75rem;align-items:center;flex-wrap:wrap;">
                    <button class="btn-save" style="padding:0.55rem 1.4rem;font-size:0.85rem;background:linear-gradient(135deg,#f59e0b,#d97706);box-shadow:0 2px 12px rgba(245,158,11,0.25);" onclick="backupImport()" id="backup-import-btn">
                        ${t('config.backup.import_button')}
                    </button>
                    <span id="backup-import-status" style="font-size:0.8rem;color:var(--text-secondary);"></span>
                </div>

                <div id="backup-import-result" style="margin-top:1rem;display:none;"></div>
            </div>`;

            html += '</div>';
            document.getElementById('content').innerHTML = html;
        }

        function backupShowTab(tab) {
            ['create', 'import'].forEach(tabId => {
                document.getElementById('backup-tab-' + tabId).classList.toggle('active', tabId === tab);
                document.getElementById('backup-panel-' + tabId).classList.toggle('active', tabId === tab);
            });
        }

        async function backupCreate() {
            const btn = document.getElementById('backup-create-btn');
            const status = document.getElementById('backup-create-status');
            btn.disabled = true;
            status.textContent = t('config.backup.creating');
            status.style.color = 'var(--text-secondary)';

            const payload = {
                password: document.getElementById('backup-password').value || '',
                include_vectordb: document.getElementById('backup-opt-vectordb').checked,
                include_workdir: document.getElementById('backup-opt-workdir').checked,
            };

            try {
                const resp = await fetch('/api/backup/create', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload)
                });
                if (!resp.ok) {
                    const txt = await resp.text();
                    throw new Error(txt);
                }
                const disposition = resp.headers.get('Content-Disposition') || '';
                const fnMatch = disposition.match(/filename="([^"]+)"/);
                const filename = fnMatch ? fnMatch[1] : 'backup.ago';
                const blob = await resp.blob();
                const url = URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url; a.download = filename;
                document.body.appendChild(a); a.click();
                document.body.removeChild(a);
                URL.revokeObjectURL(url);
                status.textContent = '✅ ' + t('config.backup.downloaded') + filename;
                status.style.color = 'var(--success)';
            } catch (e) {
                status.textContent = '❌ ' + e.message;
                status.style.color = 'var(--danger)';
            } finally {
                btn.disabled = false;
            }
        }

        async function backupImport() {
            const btn = document.getElementById('backup-import-btn');
            const status = document.getElementById('backup-import-status');
            const result = document.getElementById('backup-import-result');
            const fileInput = document.getElementById('backup-import-file');

            if (!fileInput.files || !fileInput.files[0]) {
                status.textContent = t('config.backup.select_file_warning');
                status.style.color = 'var(--warning)';
                return;
            }

            btn.disabled = true;
            status.textContent = t('config.backup.importing');
            status.style.color = 'var(--text-secondary)';
            result.style.display = 'none';

            const fd = new FormData();
            fd.append('file', fileInput.files[0]);
            fd.append('password', document.getElementById('backup-import-password').value || '');

            try {
                const resp = await fetch('/api/backup/import', { method: 'POST', body: fd });
                const data = await resp.json();
                if (!resp.ok || data.error) {
                    let msg = data.message || data.error || 'Unknown error';
                    if (data.error === 'password_required') msg = t('config.backup.password_required');
                    if (data.error === 'decryption_failed') msg = t('config.backup.decryption_failed');
                    status.textContent = '❌ ' + msg;
                    status.style.color = 'var(--danger)';
                } else {
                    status.textContent = '';
                    result.style.display = 'block';
                    result.innerHTML = `<div style="background:rgba(34,197,94,0.1);border:1px solid rgba(34,197,94,0.3);border-radius:10px;padding:1rem 1.25rem;">
                        <div style="font-weight:600;color:var(--success);margin-bottom:0.4rem;">✅ ${t('config.backup.import_success')}</div>
                        <div style="font-size:0.82rem;color:var(--text-secondary);">${data.message}</div>
                        <button class="btn-save" style="margin-top:0.85rem;padding:0.45rem 1.1rem;font-size:0.8rem;" onclick="restartAuraGo()">
                            ↻ ${t('config.backup.restart_now')}
                        </button>
                    </div>`;
                }
            } catch (e) {
                status.textContent = '❌ ' + e.message;
                status.style.color = 'var(--danger)';
            } finally {
                btn.disabled = false;
            }
        }
